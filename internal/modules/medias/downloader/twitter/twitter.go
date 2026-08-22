// Package twitter implements a Twitter/X media downloader.
package twitter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	twitterAPIURL   = "https://twitter.com/i/api/graphql/2ICDjqPd81tulZcYrtpTuQ/TweetResultByRestId"
	guestTokenURL   = "https://api.twitter.com/1.1/guest/activate.json"
	fxTwitterAPIURL = "https://api.fxtwitter.com/status/"
)

var (
	defaultHeaders = map[string]string{
		"Authorization":             "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
		"x-twitter-client-language": "en",
		"x-twitter-active-user":     "yes",
		"content-type":              "application/json",
	}

	postIDRegex = regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`)
)

func Handle(text string, opts downloader.Options) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(handler.postID); err == nil {
		return postInfo
	}

	twitterData := handler.getTwitterData()
	if twitterData == nil {
		fxTwitterData := handler.getFxTwitterData()
		if fxTwitterData != nil {
			postInfo, cleanup := handler.processFxTwitterAPI(fxTwitterData)
			postInfo.Cleanup = downloader.CombineCleanups(postInfo.Cleanup, cleanup)
			return postInfo
		}
		return downloader.NewUnavailablePostInfo(handler.postID)
	}

	handler.username = (*twitterData).Data.TweetResult.Core.UserResults.Result.Legacy.ScreenName
	postInfo, cleanup := handler.processTwitterAPI(twitterData)
	postInfo.Cleanup = downloader.CombineCleanups(postInfo.Cleanup, cleanup)
	return postInfo
}

func (h *Handler) setPostID(url string) bool {
	if matches := postIDRegex.FindStringSubmatch(url); len(matches) == 2 {
		h.postID = matches[1]
		return true
	}
	return false
}

type MediaUnavailableError struct {
	Status string
	Reason string
}

func (e *MediaUnavailableError) Error() string {
	return fmt.Sprintf("media unavailable: %s (%s)", e.Status, e.Reason)
}

func (h *Handler) processTwitterAPI(twitterData *TwitterAPIData) (downloader.PostInfo, func()) {
	var invertMedia bool

	allTweetMedia := (*twitterData).Data.TweetResult.Legacy.ExtendedEntities.Media
	if quotedStatusResult := (*twitterData).Data.TweetResult.QuotedStatusResult; len(allTweetMedia) == 0 && quotedStatusResult != nil && quotedStatusResult.Legacy != nil {
		if quotedStatusResult.Legacy.ExtendedEntities.Media != nil {
			invertMedia = true
			allTweetMedia = quotedStatusResult.Legacy.ExtendedEntities.Media
		}
	}

	mediaCount := len(allTweetMedia)
	if mediaCount == 0 {
		slog.Debug("No media found in tweet",
			"Post Info", []string{h.username, h.postID})
		return downloader.NewNoMediaPostInfo(h.postID), nil
	}

	mediaItems := make([]gotgbot.InputMedia, mediaCount)
	results := downloadAllMedia(allTweetMedia, func(_ int, twitterMedia Media) (gotgbot.InputMedia, func(), error) {
		if twitterMedia.ExtMediaAvailability.Status != "" && twitterMedia.ExtMediaAvailability.Status != "Available" {
			return nil, nil, &MediaUnavailableError{
				Status: twitterMedia.ExtMediaAvailability.Status,
				Reason: twitterMedia.ExtMediaAvailability.Reason,
			}
		}
		return h.downloadMedia(twitterMedia, true)
	})

	var cleanups []func()
	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	var unavailableReason string
	for _, result := range results {
		if result.err != nil {
			if _, ok := errors.AsType[*downloader.FileTooLargeError](result.err); ok {
				return downloader.NewFileTooLargePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
			}
			if unavailableErr, ok := errors.AsType[*MediaUnavailableError](result.err); ok {
				slog.Info("Media unavailable in carousel", "Post Info", []string{h.username, h.postID},
					"Media Count", result.index, "Error", unavailableErr.Error())
				if unavailableReason == "" {
					unavailableReason = unavailableErr.Reason
				}
			} else {
				slog.Error("Failed to download media in carousel", "Post Info", []string{h.username, h.postID},
					"Media Count", result.index, "Error", result.err.Error())
			}
			continue
		}
		addCleanup(result.cleanup)
		if result.media != nil {
			mediaItems[result.index] = result.media
		}
	}

	nonNil := make([]gotgbot.InputMedia, 0, len(mediaItems))
	for _, m := range mediaItems {
		if m != nil {
			nonNil = append(nonNil, m)
		}
	}

	if len(nonNil) == 0 {
		if unavailableReason != "" {
			return downloader.NewUnavailablePostInfoWithReason(h.postID, unavailableReason), downloader.CombineCleanups(cleanups...)
		}
		return downloader.NewUnavailablePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
	}

	var article *downloader.ArticleContent
	article, articleCleanups := h.tryBuildReplyArticle(twitterData, nonNil)
	if len(articleCleanups) > 0 {
		cleanups = append(cleanups, articleCleanups...)
	}

	return downloader.PostInfo{
		Medias:      nonNil,
		ID:          h.postID,
		Caption:     getTweetCaption(twitterData),
		InvertMedia: invertMedia,
		Article:     article,
	}, downloader.CombineCleanups(cleanups...)
}

type mediaResult struct {
	index   int
	media   gotgbot.InputMedia
	cleanup func()
	err     error
}

func downloadAllMedia[T any](items []T, download func(index int, item T) (gotgbot.InputMedia, func(), error)) []mediaResult {
	results := make(chan mediaResult, len(items))
	for i, item := range items {
		go func(index int, it T) {
			media, cleanup, err := download(index, it)
			results <- mediaResult{index: index, media: media, cleanup: cleanup, err: err}
		}(i, item)
	}
	out := make([]mediaResult, 0, len(items))
	for range len(items) {
		out = append(out, <-results)
	}
	return out
}

func (h *Handler) downloadResultMedia(result Result) ([]gotgbot.InputMedia, []func()) {
	if result.Legacy == nil || len(result.Legacy.ExtendedEntities.Media) == 0 {
		return nil, nil
	}
	results := downloadAllMedia(result.Legacy.ExtendedEntities.Media, func(_ int, m Media) (gotgbot.InputMedia, func(), error) {
		if m.ExtMediaAvailability.Status != "" && m.ExtMediaAvailability.Status != "Available" {
			return nil, nil, &MediaUnavailableError{Status: m.ExtMediaAvailability.Status, Reason: m.ExtMediaAvailability.Reason}
		}
		return h.downloadMedia(m, false)
	})
	ordered := make([]gotgbot.InputMedia, len(results))
	var cleanups []func()
	for _, res := range results {
		if res.err != nil {
			slog.Info("Failed to download referenced tweet media",
				"Post Info", []string{h.username, h.postID},
				"Media Count", res.index, "Error", res.err.Error())
			continue
		}
		if res.cleanup != nil {
			cleanups = append(cleanups, res.cleanup)
		}
		if res.media != nil {
			ordered[res.index] = res.media
		}
	}
	var medias []gotgbot.InputMedia
	for _, m := range ordered {
		if m != nil {
			medias = append(medias, m)
		}
	}
	return medias, cleanups
}

func writeTweetHeaderAndText(sb *strings.Builder, tweet Result) {
	fmt.Fprintf(sb, "<p><b><a href=\"https://x.com/%s\">%s</a> (<code>@%s</code>)</b></p>\n",
		tweet.Core.UserResults.Result.Legacy.ScreenName,
		escapeText(tweet.Core.UserResults.Result.Legacy.Name),
		escapeText(tweet.Core.UserResults.Result.Legacy.ScreenName))
	text := cleanText(tweet.Legacy.FullText, tweet.Legacy)
	if tweet.NoteTweet != nil {
		text = tweet.NoteTweet.NoteTweetResults.Result.Text
	}
	downloader.AppendCaptionParagraph(sb, escapeText(text))
}

func (h *Handler) tryBuildReplyArticle(mainData *TwitterAPIData, mainMedias []gotgbot.InputMedia) (*downloader.ArticleContent, []func()) {
	tweet := (*mainData).Data.TweetResult.Result
	if tweet.Legacy == nil {
		return nil, nil
	}

	var (
		parentResult       Result
		parentMedias       []gotgbot.InputMedia
		parentCleanups     []func()
		quoteMediaPromoted bool
	)

	if quoted := tweet.QuotedStatusResult; quoted != nil && quoted.Result.Legacy != nil && quoted.Result.Core.UserResults.Result.Legacy != nil {
		parentResult = quoted.Result
		if len(tweet.Legacy.ExtendedEntities.Media) > 0 {
			quotedHandler := &Handler{username: parentResult.Core.UserResults.Result.Legacy.ScreenName, postID: parentResult.RestID}
			parentMedias, parentCleanups = quotedHandler.downloadResultMedia(parentResult)
		} else {
			quoteMediaPromoted = true
		}
	} else {
		return nil, nil
	}

	var htmlBuilder strings.Builder
	var mediaList []gotgbot.InputRichMessageMedia

	writeTweetHeaderAndText(&htmlBuilder, tweet)

	if !quoteMediaPromoted {
		mediaList = downloader.AppendRichMedia(&htmlBuilder, mainMedias, 1)
	}

	htmlBuilder.WriteString("<blockquote>\n")

	writeTweetHeaderAndText(&htmlBuilder, parentResult)

	if quoteMediaPromoted {
		mediaList = downloader.AppendRichMedia(&htmlBuilder, mainMedias, 1)
	} else {
		mediaList = append(mediaList, downloader.AppendRichMedia(&htmlBuilder, parentMedias, len(mediaList)+1)...)
	}

	htmlBuilder.WriteString("</blockquote>\n")

	return &downloader.ArticleContent{HTML: htmlBuilder.String(), Media: mediaList}, parentCleanups
}

const maxVideoSize int64 = 500 * 1024 * 1024 // 500MB

func (h *Handler) downloadMedia(twitterMedia Media, showCaptionAbove bool) (gotgbot.InputMedia, func(), error) {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Twitter_%s_%s", h.username, h.postID))

	if slices.Contains([]string{"animated_gif", "video"}, twitterMedia.Type) {
		sort.Slice(twitterMedia.VideoInfo.Variants, func(i, j int) bool {
			return twitterMedia.VideoInfo.Variants[i].Bitrate < twitterMedia.VideoInfo.Variants[j].Bitrate
		})
		videoURL := twitterMedia.VideoInfo.Variants[len(twitterMedia.VideoInfo.Variants)-1].URL

		if size, err := downloader.FetchSizeFromURL(videoURL); err == nil && size > 0 && size > maxVideoSize {
			return nil, nil, downloader.NewFileTooLargeError(size, maxVideoSize)
		}

		stream, cleanup, err := downloader.FetchStreamFromURL(videoURL)
		if err != nil {
			return nil, nil, err
		}

		videoMedia := &gotgbot.InputMediaVideo{
			Media:                 downloader.InputFileFromReader(filename, stream),
			ShowCaptionAboveMedia: showCaptionAbove,
			Width:                 int64(twitterMedia.OriginalInfo.Width),
			Height:                int64(twitterMedia.OriginalInfo.Height),
			SupportsStreaming:     true,
		}

		if twitterMedia.MediaURLHTTPS != "" {
			thumbnail, thumbErr := downloader.FetchBytesFromURL(twitterMedia.MediaURLHTTPS)
			if thumbErr != nil {
				cleanup()
				return nil, nil, thumbErr
			}
			if resized, err := utils.ResizeThumbnail(thumbnail); err == nil {
				videoMedia.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(resized))
			} else {
				slog.Error("Failed to resize thumbnail",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error())
			}
		}

		return videoMedia, cleanup, nil
	}

	stream, cleanup, err := downloader.FetchStreamFromURL(twitterMedia.MediaURLHTTPS)
	if err != nil {
		return nil, nil, err
	}

	return &gotgbot.InputMediaPhoto{Media: downloader.InputFileFromReader(filename, stream), ShowCaptionAboveMedia: showCaptionAbove}, cleanup, nil
}

func (h *Handler) getGuestToken() string {
	if token, err := cache.GetCache("twitter_guest_token"); token != "" && err == nil {
		return token
	}

	type guestToken struct {
		GuestToken string `json:"guest_token"`
	}
	var res guestToken

	retryCaller := downloader.LongRetryCaller()

	response, err := retryCaller.Request(guestTokenURL, utils.RequestParams{
		Method:    "POST",
		Headers:   defaultHeaders,
		Redirects: 3,
	})

	if err != nil {
		slog.Error("Failed to get guest token",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return ""
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil {
		slog.Error("Failed to unmarshal guest token",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return ""
	}

	cache.SetCache("twitter_guest_token", res.GuestToken, 3*time.Hour)
	return res.GuestToken
}

func (h *Handler) getTwitterData() *TwitterAPIData {
	guestToken := h.getGuestToken()
	if guestToken == "" {
		return nil
	}

	headers := make(map[string]string, len(defaultHeaders)+2)
	for key, value := range defaultHeaders {
		headers[key] = value
	}
	headers["x-guest-token"] = guestToken
	headers["cookie"] = fmt.Sprintf("guest_id=v1:%v;", guestToken)

	variables := map[string]any{
		"tweetId":                h.postID,
		"includePromotedContent": false,
		"withCommunity":          false,
		"withVoice":              false,
	}

	features := map[string]any{
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"tweetypie_unmention_optimization_enabled":                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                false,
		"tweet_awards_web_tipping_enabled":                                        false,
		"responsive_web_home_pinned_timelines_enabled":                            true,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"responsive_web_media_download_video_enabled":                             false,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}

	fieldToggles := map[string]any{
		"withArticleRichContentState": true,
	}

	jsonMarshal := func(data any) []byte {
		result, _ := json.Marshal(data)
		return result
	}

	retryCaller := downloader.LongRetryCaller()

	response, err := retryCaller.Request(twitterAPIURL, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"variables":    string(jsonMarshal(variables)),
			"features":     string(jsonMarshal(features)),
			"fieldToggles": string(jsonMarshal(fieldToggles)),
		},
		Headers: headers,
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	var twitterAPIData *TwitterAPIData
	err = json.NewDecoder(response.Body).Decode(&twitterAPIData)
	if err != nil {
		return nil
	}

	if twitterAPIData != nil &&
		(*twitterAPIData).Data.TweetResult != nil &&
		(*twitterAPIData).Data.TweetResult.Reason != nil &&
		*(*twitterAPIData).Data.TweetResult.Reason == "NsfwLoggedOut" {
		return nil
	}

	if twitterAPIData == nil ||
		(*twitterAPIData).Data.TweetResult == nil ||
		(*twitterAPIData).Data.TweetResult.Legacy == nil {
		return nil
	}

	return twitterAPIData
}

func (h *Handler) getFxTwitterData() *FxTwitterAPIData {
	retryCaller := downloader.LongRetryCaller()

	response, err := retryCaller.Request(fxTwitterAPIURL+h.postID, utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})
	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	var fxTwitterAPIData *FxTwitterAPIData
	err = json.NewDecoder(response.Body).Decode(&fxTwitterAPIData)
	if err != nil {
		return nil
	}

	if fxTwitterAPIData == nil || fxTwitterAPIData.Code != 200 {
		return nil
	}

	return fxTwitterAPIData
}

func (h *Handler) processFxTwitterAPI(twitterData *FxTwitterAPIData) (downloader.PostInfo, func()) {
	var (
		allMedia          []gotgbot.InputMedia
		quoteMedia        []gotgbot.InputMedia
		cleanups          []func()
		quoteMediaPresent bool
	)

	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	if twitterData.Tweet.Media != nil && len(twitterData.Tweet.Media.All) > 0 {
		results := downloadAllMedia(twitterData.Tweet.Media.All, func(index int, twitterMedia FxTwitterMedia) (gotgbot.InputMedia, func(), error) {
			return h.downloadFxMedia(twitterMedia, index)
		})
		var mediaItems []gotgbot.InputMedia
		for _, result := range results {
			if result.err != nil {
				if _, ok := errors.AsType[*downloader.FileTooLargeError](result.err); ok {
					return downloader.NewFileTooLargePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
				}
				slog.Error("Failed to download media in carousel",
					"Post Info", []string{h.username, h.postID},
					"Media Count", result.index, "Error", result.err.Error())
				continue
			}
			addCleanup(result.cleanup)
			if result.media != nil {
				mediaItems = append(mediaItems, result.media)
			}
		}
		allMedia = mediaItems
	}

	if twitterData.Tweet.Quote != nil && twitterData.Tweet.Quote.Media != nil && len(twitterData.Tweet.Quote.Media.All) > 0 {
		quoteMediaPresent = true
		results := downloadAllMedia(twitterData.Tweet.Quote.Media.All, func(index int, twitterMedia FxTwitterMedia) (gotgbot.InputMedia, func(), error) {
			return h.downloadFxMedia(twitterMedia, index)
		})
		var mediaItems []gotgbot.InputMedia
		for _, result := range results {
			if result.err != nil {
				if _, ok := errors.AsType[*downloader.FileTooLargeError](result.err); ok {
					return downloader.NewFileTooLargePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
				}
				slog.Error("Failed to download media in carousel",
					"Post Info", []string{h.username, h.postID},
					"Media Count", result.index, "Error", result.err.Error())
				continue
			}
			addCleanup(result.cleanup)
			if result.media != nil {
				mediaItems = append(mediaItems, result.media)
			}
		}
		quoteMedia = mediaItems
	}

	if len(allMedia) == 0 && len(quoteMedia) == 0 {
		slog.Debug("No media found in tweet (fxTwitter)",
			"Post ID", h.postID)
		return downloader.NewNoMediaPostInfo(h.postID), downloader.CombineCleanups(cleanups...)
	}

	var article *downloader.ArticleContent
	if twitterData.Tweet.Quote != nil {
		article = buildFxReplyArticle(twitterData.Tweet, allMedia, twitterData.Tweet.Quote, quoteMedia)
	}

	medias := allMedia
	if len(medias) == 0 {
		medias = quoteMedia
	}

	return downloader.PostInfo{
		Medias:      medias,
		ID:          h.postID,
		Caption:     getFxTweetCaption(twitterData),
		InvertMedia: quoteMediaPresent && len(allMedia) == 0,
		Article:     article,
	}, downloader.CombineCleanups(cleanups...)
}

func (h *Handler) downloadFxMedia(twitterMedia FxTwitterMedia, index int) (gotgbot.InputMedia, func(), error) {
	if twitterMedia.Type == "video" {
		if size, sizeErr := downloader.FetchSizeFromURL(twitterMedia.URL); sizeErr == nil && size > 0 && size > maxVideoSize {
			return nil, nil, downloader.NewFileTooLargeError(size, maxVideoSize)
		}
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Twitter_%s_%s_%d", h.username, h.postID, index))
	if twitterMedia.Type == "video" {
		stream, cleanup, err := downloader.FetchStreamFromURL(twitterMedia.URL)
		if err != nil {
			return nil, nil, err
		}

		videoMedia := &gotgbot.InputMediaVideo{
			Media:                 downloader.InputFileFromReader(filename, stream),
			ShowCaptionAboveMedia: true,
			Width:                 int64(twitterMedia.Width),
			Height:                int64(twitterMedia.Height),
			SupportsStreaming:     true,
		}

		if twitterMedia.ThumbnailURL != "" {
			thumbnail, thumbErr := downloader.FetchBytesFromURL(twitterMedia.ThumbnailURL)
			if thumbErr != nil {
				cleanup()
				return nil, nil, thumbErr
			}
			if resized, err := utils.ResizeThumbnail(thumbnail); err == nil {
				videoMedia.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(resized))
			} else {
				slog.Error("Failed to resize thumbnail",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error())
			}
		}

		return videoMedia, cleanup, nil
	}

	stream, cleanup, err := downloader.FetchStreamFromURL(twitterMedia.URL)
	if err != nil {
		return nil, nil, err
	}

	return &gotgbot.InputMediaPhoto{Media: downloader.InputFileFromReader(filename, stream), ShowCaptionAboveMedia: true}, cleanup, nil
}

func buildFxReplyArticle(tweet FxTwitterTweet, mainMedias []gotgbot.InputMedia, quote *FxTwitterTweet, quoteMedias []gotgbot.InputMedia) *downloader.ArticleContent {
	var htmlBuilder strings.Builder
	var mediaList []gotgbot.InputRichMessageMedia

	writeFxTweetHeaderAndText(&htmlBuilder, tweet)

	if len(mainMedias) > 0 {
		mediaList = downloader.AppendRichMedia(&htmlBuilder, mainMedias, 1)
	}

	if quote != nil {
		htmlBuilder.WriteString("<blockquote>\n")
		writeFxTweetHeaderAndText(&htmlBuilder, *quote)
		if len(quoteMedias) > 0 {
			mediaList = append(mediaList, downloader.AppendRichMedia(&htmlBuilder, quoteMedias, len(mediaList)+1)...)
		}
		htmlBuilder.WriteString("</blockquote>\n")
	}

	return &downloader.ArticleContent{HTML: htmlBuilder.String(), Media: mediaList}
}

func writeFxTweetHeaderAndText(sb *strings.Builder, tweet FxTwitterTweet) {
	fmt.Fprintf(sb, "<p><b><a href=\"https://x.com/%s\">%s</a> (<code>@%s</code>)</b></p>\n",
		tweet.Author.ScreenName,
		escapeText(tweet.Author.Name),
		escapeText(tweet.Author.ScreenName))
	downloader.AppendCaptionParagraph(sb, escapeText(tweet.Text))
}

func escapeText(text string) string {
	return html.EscapeString(html.UnescapeString(text))
}

func replaceExpandedURLs(text string, urls []struct {
	URL         string `json:"url"`
	ExpandedURL string `json:"expanded_url"`
}) string {
	if len(urls) == 0 || text == "" {
		return text
	}

	replaced := text
	for _, u := range urls {
		if u.URL == "" || u.ExpandedURL == "" {
			continue
		}
		replaced = strings.ReplaceAll(replaced, u.URL, u.ExpandedURL)
	}
	return replaced
}

func trimTrailingTCo(text string) string {
	trimmed := strings.TrimRight(text, " \n\r\t")
	for strings.HasPrefix(trimmed, "https://t.co/") {
		if idx := strings.LastIndexAny(trimmed, " \n\r\t"); idx != -1 {
			trimmed = strings.TrimRight(trimmed[:idx], " \n\r\t")
			continue
		}
		return ""
	}

	if idx := strings.LastIndex(trimmed, "https://t.co/"); idx != -1 {
		suffix := trimmed[idx:]
		if !strings.ContainsAny(suffix, " \n\r\t") {
			if idx > 0 {
				trimmed = strings.TrimRight(trimmed[:idx], " \n\r\t")
			} else {
				trimmed = ""
			}
		}
	}

	return trimmed
}

func cleanText(text string, legacy Legacy) string {
	expanded := replaceExpandedURLs(text, legacy.Entities.Urls)
	return trimTrailingTCo(expanded)
}

func getTweetCaption(twitterData *TwitterAPIData) string {
	const maxQuotedTextRunes = 248

	tweet := (*twitterData).Data.TweetResult.Result
	if tweet.Legacy == nil {
		return ""
	}

	var caption strings.Builder

	tweetText := tweet.Legacy.FullText
	quotedStatusResult := (*twitterData).Data.TweetResult.QuotedStatusResult

	if tweet.NoteTweet != nil && quotedStatusResult == nil {
		tweetText = tweet.NoteTweet.NoteTweetResults.Result.Text
	}

	fmt.Fprintf(&caption, "<b><a href=\"https://x.com/%s\">%s</a> (<code>@%s</code>)</b>:\n%s",
		tweet.Core.UserResults.Result.Legacy.ScreenName,
		escapeText(tweet.Core.UserResults.Result.Legacy.Name),
		escapeText(tweet.Core.UserResults.Result.Legacy.ScreenName),
		escapeText(cleanText(tweetText, tweet.Legacy)))

	if quotedStatusResult != nil && quotedStatusResult.Legacy != nil && quotedStatusResult.Core.UserResults.Result.Legacy != nil {
		quotedText := cleanText(quotedStatusResult.Legacy.FullText, quotedStatusResult.Legacy)
		if utf8.RuneCountInString(quotedText) > maxQuotedTextRunes {
			runes := []rune(quotedText)
			quotedText = string(runes[:maxQuotedTextRunes]) + "\n..."
		}

		fmt.Fprintf(&caption, "\n<blockquote><i>Quoting</i> <b><a href=\"https://x.com/%s\">%s</a> (<code>@%s</code>)</b>:\n%s</blockquote>",
			quotedStatusResult.Core.UserResults.Result.Legacy.ScreenName,
			escapeText(quotedStatusResult.Core.UserResults.Result.Legacy.Name),
			escapeText(quotedStatusResult.Core.UserResults.Result.Legacy.ScreenName),
			escapeText(quotedText))
	}

	return caption.String()
}

func getFxTweetCaption(twitterData *FxTwitterAPIData) string {
	const maxQuotedTextRunes = 248

	var caption strings.Builder

	fmt.Fprintf(&caption, "<b><a href=\"https://x.com/%s\">%s</a> (<code>@%s</code>)</b>:\n%s",
		twitterData.Tweet.Author.ScreenName,
		escapeText(twitterData.Tweet.Author.Name),
		escapeText(twitterData.Tweet.Author.ScreenName),
		escapeText(twitterData.Tweet.Text))

	if twitterData.Tweet.Quote != nil {
		quotedText := twitterData.Tweet.Quote.Text
		if utf8.RuneCountInString(quotedText) > maxQuotedTextRunes {
			runes := []rune(quotedText)
			quotedText = string(runes[:maxQuotedTextRunes]) + "\n..."
		}

		fmt.Fprintf(&caption, "\n<blockquote><i>Quoting</i> <b><a href=\"https://x.com/%s\">%s</a> (<code>@%s</code>)</b>:\n%s</blockquote>",
			twitterData.Tweet.Quote.Author.ScreenName,
			escapeText(twitterData.Tweet.Quote.Author.Name),
			escapeText(twitterData.Tweet.Quote.Author.ScreenName),
			escapeText(quotedText))
	}

	return caption.String()
}
