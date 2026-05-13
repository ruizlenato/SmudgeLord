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
)

func Handle(text string) downloader.PostInfo {
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
	if matches := regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`).FindStringSubmatch(url); len(matches) == 2 {
		h.postID = matches[1]
		return true
	}
	return false
}

func (h *Handler) processTwitterAPI(twitterData *TwitterAPIData) (downloader.PostInfo, func()) {
	type mediaResult struct {
		index   int
		media   gotgbot.InputMedia
		cleanup func()
		err     error
	}

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
	results := make(chan mediaResult, mediaCount)

	for i, media := range allTweetMedia {
		go func(index int, twitterMedia Media) {
			media, cleanup, err := h.downloadMedia(twitterMedia, invertMedia && twitterMedia.Type != "video")
			results <- mediaResult{index: index, media: media, cleanup: cleanup, err: err}
		}(i, media)
	}

	var cleanups []func()
	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	for range mediaCount {
		result := <-results
		if result.err != nil {
			var fileTooLargeErr *downloader.FileTooLargeError
			if errors.As(result.err, &fileTooLargeErr) {
				return downloader.NewFileTooLargePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
			}
			slog.Error("Failed to download media in carousel", "Post Info", []string{h.username, h.postID},
				"Media Count", result.index, "Error", result.err.Error())
			continue
		}
		addCleanup(result.cleanup)
		if result.media != nil {
			mediaItems[result.index] = result.media
		}
	}

	if !slices.ContainsFunc(mediaItems, func(m gotgbot.InputMedia) bool { return m != nil }) {
		return downloader.NewUnavailablePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
	}

	return downloader.PostInfo{
		Medias:      mediaItems,
		ID:          h.postID,
		Caption:     getTweetCaption(twitterData),
		InvertMedia: invertMedia,
	}, downloader.CombineCleanups(cleanups...)
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
			if thumbnail, err := utils.ResizeThumbnail(thumbnail); err == nil {
				videoMedia.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(thumbnail))
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

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   5 * time.Second,
		MaxDelay:     10 * time.Second,
	}

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

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   5 * time.Second,
		MaxDelay:     10 * time.Second,
	}

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
	response, err := utils.Request(fxTwitterAPIURL+h.postID, utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})
	if err != nil || response.Body == nil {
		return nil
	}

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
	type mediaResult struct {
		index   int
		media   gotgbot.InputMedia
		cleanup func()
		err     error
	}

	var allMedia []FxTwitterMedia
	var invertMedia bool
	if twitterData.Tweet.Media != nil && len(twitterData.Tweet.Media.All) > 0 {
		allMedia = twitterData.Tweet.Media.All
	} else if twitterData.Tweet.Quote != nil && twitterData.Tweet.Quote.Media != nil && len(twitterData.Tweet.Quote.Media.All) > 0 {
		invertMedia = true
		allMedia = twitterData.Tweet.Quote.Media.All
	} else {
		slog.Debug("No media found in tweet (fxTwitter)",
			"Post ID", h.postID)
		return downloader.NewNoMediaPostInfo(h.postID), nil
	}

	mediaCount := len(allMedia)
	mediaItems := make([]gotgbot.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range allMedia {
		go func(index int, twitterMedia FxTwitterMedia) {
			if twitterMedia.Type == "video" {
				if size, sizeErr := downloader.FetchSizeFromURL(twitterMedia.URL); sizeErr == nil && size > 0 && size > maxVideoSize {
					results <- mediaResult{index: index, err: downloader.NewFileTooLargeError(size, maxVideoSize)}
					return
				}
			}

			filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Twitter_%s_%s_%d", h.username, h.postID, index))
			if twitterMedia.Type == "video" {
				stream, cleanup, err := downloader.FetchStreamFromURL(twitterMedia.URL)
				if err != nil {
					results <- mediaResult{index: index, err: err}
					return
				}

				videoMedia := &gotgbot.InputMediaVideo{
					Media:                 downloader.InputFileFromReader(filename, stream),
					ShowCaptionAboveMedia: invertMedia,
					Width:                 int64(twitterMedia.Width),
					Height:                int64(twitterMedia.Height),
					SupportsStreaming:     true,
				}

				if twitterMedia.ThumbnailURL != "" {
					thumbnail, thumbErr := downloader.FetchBytesFromURL(twitterMedia.ThumbnailURL)
					if thumbErr != nil {
						cleanup()
						results <- mediaResult{index: index, err: thumbErr}
						return
					}
					if thumbnail, err := utils.ResizeThumbnail(thumbnail); err == nil {
						videoMedia.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(thumbnail))
					} else {
						slog.Error("Failed to resize thumbnail",
							"Post Info", []string{h.username, h.postID},
							"Error", err.Error())
					}
				}

				results <- mediaResult{index: index, media: videoMedia, cleanup: cleanup}
				return
			}

			stream, cleanup, err := downloader.FetchStreamFromURL(twitterMedia.URL)
			if err != nil {
				results <- mediaResult{index: index, err: err}
				return
			}

			results <- mediaResult{index: index, media: &gotgbot.InputMediaPhoto{Media: downloader.InputFileFromReader(filename, stream)}, cleanup: cleanup}
		}(i, media)
	}

	var cleanups []func()
	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	for range mediaCount {
		result := <-results
		if result.err != nil {
			var fileTooLargeErr *downloader.FileTooLargeError
			if errors.As(result.err, &fileTooLargeErr) {
				return downloader.NewFileTooLargePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
			}
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index, "Error", result.err.Error())
			continue
		}
		addCleanup(result.cleanup)
		if result.media != nil {
			mediaItems[result.index] = result.media
		}
	}

	if !slices.ContainsFunc(mediaItems, func(m gotgbot.InputMedia) bool { return m != nil }) {
		return downloader.NewUnavailablePostInfo(h.postID), downloader.CombineCleanups(cleanups...)
	}

	return downloader.PostInfo{
		Medias:      mediaItems,
		ID:          h.postID,
		Caption:     getFxTweetCaption(twitterData),
		InvertMedia: invertMedia,
	}, downloader.CombineCleanups(cleanups...)
}

func getTweetCaption(twitterData *TwitterAPIData) string {
	const maxQuotedTextRunes = 248

	tweet := (*twitterData).Data.TweetResult.Result
	if tweet.Legacy == nil {
		return ""
	}

	escapeTelegramText := func(text string) string {
		return html.EscapeString(html.UnescapeString(text))
	}

	replaceExpandedURLs := func(text string, urls []struct {
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

	trimTrailingTCo := func(text string) string {
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

	cleanText := func(text string, legacy Legacy) string {
		expanded := replaceExpandedURLs(text, legacy.Entities.Urls)
		return trimTrailingTCo(expanded)
	}

	var caption strings.Builder

	tweetText := tweet.Legacy.FullText
	quotedStatusResult := (*twitterData).Data.TweetResult.QuotedStatusResult

	if tweet.NoteTweet != nil && quotedStatusResult == nil {
		tweetText = tweet.NoteTweet.NoteTweetResults.Result.Text
	}

	fmt.Fprintf(&caption, "<b>%s (<code>%s</code>)</b>:\n%s",
		escapeTelegramText(tweet.Core.UserResults.Result.Legacy.Name),
		escapeTelegramText(tweet.Core.UserResults.Result.Legacy.ScreenName),
		escapeTelegramText(cleanText(tweetText, tweet.Legacy)))

	if quotedStatusResult != nil && quotedStatusResult.Legacy != nil && quotedStatusResult.Core.UserResults.Result.Legacy != nil {
		quotedText := cleanText(quotedStatusResult.Legacy.FullText, quotedStatusResult.Legacy)
		if utf8.RuneCountInString(quotedText) > maxQuotedTextRunes {
			runes := []rune(quotedText)
			quotedText = string(runes[:maxQuotedTextRunes]) + "\n..."
		}

		fmt.Fprintf(&caption, "\n<blockquote><i>Quoting</i> <b>%s (<code>%s</code>)</b>:\n%s</blockquote>",
			escapeTelegramText(quotedStatusResult.Core.UserResults.Result.Legacy.Name),
			escapeTelegramText(quotedStatusResult.Core.UserResults.Result.Legacy.ScreenName),
			escapeTelegramText(quotedText))
	}

	return caption.String()
}

func getFxTweetCaption(twitterData *FxTwitterAPIData) string {
	const maxQuotedTextRunes = 248

	var caption strings.Builder

	escapeTelegramText := func(text string) string {
		return html.EscapeString(html.UnescapeString(text))
	}

	fmt.Fprintf(&caption, "<b>%s (<code>%s</code>)</b>:\n%s",
		escapeTelegramText(twitterData.Tweet.Author.Name),
		escapeTelegramText(twitterData.Tweet.Author.ScreenName),
		escapeTelegramText(twitterData.Tweet.Text))

	if twitterData.Tweet.Quote != nil {
		quotedText := twitterData.Tweet.Quote.Text
		if utf8.RuneCountInString(quotedText) > maxQuotedTextRunes {
			runes := []rune(quotedText)
			quotedText = string(runes[:maxQuotedTextRunes]) + "\n..."
		}

		fmt.Fprintf(&caption, "\n<blockquote><i>Quoting</i> <b>%s (<code>%s</code>)</b>:\n%s</blockquote>",
			escapeTelegramText(twitterData.Tweet.Quote.Author.Name),
			escapeTelegramText(twitterData.Tweet.Quote.Author.ScreenName),
			escapeTelegramText(quotedText))
	}

	return caption.String()
}
