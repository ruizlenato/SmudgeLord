package twitter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	twitterAPIURL = "https://twitter.com/i/api/graphql/2ICDjqPd81tulZcYrtpTuQ/TweetResultByRestId"
	guestTokenURL = "https://api.twitter.com/1.1/guest/activate.json"
)

func defaultHeaders() map[string]string {
	downloader.GenericHeaders["Authorization"] = "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"
	downloader.GenericHeaders["x-twitter-client-language"] = "en"
	downloader.GenericHeaders["x-twitter-active-user"] = "yes"
	downloader.GenericHeaders["content-type"] = "application/json"
	return downloader.GenericHeaders
}

type Handler struct {
	username string
	postID   string
}

func Handle(text string) ([]telego.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, handler.postID}
	}

	twitterData := handler.getTwitterData()
	if twitterData == nil {
		fxTwitterData := handler.getFxTwitterData()
		if fxTwitterData != nil {
			handler.username = fxTwitterData.Tweet.Author.ScreenName
			medias, caption := handler.processFxTwitterAPI(fxTwitterData)
			return medias, []string{caption, handler.postID}
		}
		return nil, []string{}
	}

	handler.username = (*twitterData).Data.TweetResult.Core.UserResults.Result.Legacy.ScreenName
	return handler.processTwitterAPI(twitterData), []string{getCaption(twitterData), handler.postID}
}

func (h *Handler) setPostID(url string) bool {
	if matches := regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`).FindStringSubmatch(url); len(matches) == 2 {
		h.postID = matches[1]
		return true
	}

	return false
}

type InputMedia struct {
	File      *os.File
	Thumbnail *os.File
}

func (h *Handler) processTwitterAPI(twitterData *TwitterAPIData) []telego.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	var quoted bool

	mediasEntities := (*twitterData).Data.TweetResult.Legacy.ExtendedEntities.Media
	if len(mediasEntities) == 0 {
		quoted = true
		if (*twitterData).Data.TweetResult.QuotedStatusResult == nil {
			return nil
		}
		mediasEntities = (*twitterData).Data.TweetResult.QuotedStatusResult.Result.Legacy.ExtendedEntities.Media
	}

	mediaCount := len(mediasEntities)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range mediasEntities {
		go func(index int, twitterMedia Media) {
			media, err := h.downloadMedia(index, twitterMedia)
			results <- mediaResult{index: index, media: media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index,
				"Error", result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if mediasEntities[result.index].Type == "photo" {
				mediaItem = &telego.InputMediaPhoto{
					Type:                  telego.MediaTypePhoto,
					Media:                 telego.InputFile{File: result.media.File},
					ShowCaptionAboveMedia: quoted,
				}
			} else {
				mediaItem = &telego.InputMediaVideo{
					Type:                  telego.MediaTypeVideo,
					Media:                 telego.InputFile{File: result.media.File},
					ShowCaptionAboveMedia: quoted,
					Width:                 (mediasEntities)[result.index].OriginalInfo.Width,
					Height:                (mediasEntities)[result.index].OriginalInfo.Height,
					SupportsStreaming:     true,
				}
				if result.media.Thumbnail != nil {
					err := utils.ResizeThumbnail(result.media.Thumbnail)
					if err != nil {
						slog.Error("Failed to resize thumbnail",
							"Post Info", []string{h.username, h.postID},
							"Error", err.Error())
					}
					mediaItem.(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: result.media.Thumbnail}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func (h *Handler) downloadMedia(index int, twitterMedia Media) (*InputMedia, error) {
	var media InputMedia
	var err error
	filename := fmt.Sprintf("SmudgeLord-Twitter_%d_%s_%s", index, h.username, h.postID)

	if slices.Contains([]string{"animated_gif", "video"}, twitterMedia.Type) {
		sort.Slice(twitterMedia.VideoInfo.Variants, func(i, j int) bool {
			return twitterMedia.VideoInfo.Variants[i].Bitrate < twitterMedia.VideoInfo.Variants[j].Bitrate
		})
		media.File, err = downloader.Downloader(twitterMedia.VideoInfo.Variants[len(twitterMedia.VideoInfo.Variants)-1].URL, filename)
		if err == nil {
			media.Thumbnail, _ = downloader.Downloader(twitterMedia.MediaURLHTTPS)
		}
	} else {
		media.File, err = downloader.Downloader(twitterMedia.MediaURLHTTPS, filename)
	}

	if err != nil {
		return nil, err
	}
	return &media, nil
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
		Caller:       utils.DefaultFastHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   5 * time.Second,
		MaxDelay:     10 * time.Second,
	}

	request, response, err := retryCaller.Request(guestTokenURL, utils.RequestParams{
		Method:    "POST",
		Headers:   defaultHeaders(),
		Redirects: 3,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		slog.Error("Failed to get guest token",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return ""
	}

	err = json.Unmarshal(response.Body(), &res)
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

	headers := defaultHeaders()

	headers["x-guest-token"] = guestToken
	headers["cookie"] = fmt.Sprintf("guest_id=v1:%v;", guestToken)

	variables := map[string]interface{}{
		"tweetId":                h.postID,
		"includePromotedContent": false,
		"withCommunity":          false,
		"withVoice":              false,
	}

	features := map[string]interface{}{
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

	fieldToggles := map[string]interface{}{
		"withArticleRichContentState": true,
	}

	jsonMarshal := func(data interface{}) []byte {
		result, _ := json.Marshal(data)
		return result
	}

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultFastHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   5 * time.Second,
		MaxDelay:     10 * time.Second,
	}

	request, response, err := retryCaller.Request(twitterAPIURL, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"variables":    string(jsonMarshal(variables)),
			"features":     string(jsonMarshal(features)),
			"fieldToggles": string(jsonMarshal(fieldToggles)),
		},
		Headers: headers,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response.Body() == nil {
		return nil
	}

	var twitterAPIData *TwitterAPIData
	err = json.Unmarshal(response.Body(), &twitterAPIData)
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

func getCaption(twitterData *TwitterAPIData) string {
	if twitterData == nil {
		return ""
	}

	tweetResult := (*twitterData).Data.TweetResult
	if tweetResult.Legacy == nil {
		return ""
	}

	user := tweetResult.Core.UserResults.Result.Legacy
	tweet := tweetResult.Legacy

	formatUser := func(name, screenName string) string {
		return fmt.Sprintf("<b>%s (<code>%s</code>):</b>\n", name, screenName)
	}

	extractText := func(text string) string {
		if idx := strings.LastIndex(text, " https://t.co/"); idx != -1 {
			return text[:idx]
		}
		if strings.HasPrefix(text, "https://t.co/") {
			return ""
		}
		return text
	}

	caption := formatUser(user.Name, user.ScreenName)

	text := tweet.FullText
	if tweetResult.NoteTweet != nil {
		text = tweetResult.NoteTweet.NoteTweetResults.Result.Text
	}
	caption += extractText(text)

	if len(tweet.ExtendedEntities.Media) == 0 {
		quoted := tweetResult.QuotedStatusResult
		if quoted != nil && quoted.Result.Legacy != nil {
			quotedTweet := quoted.Result.Legacy
			quotedUser := quoted.Result.Core.UserResults.Result.Legacy

			caption += fmt.Sprintf("\n<blockquote expandable><i>Quoting</i> %s%s</blockquote>\n",
				formatUser(quotedUser.Name, quotedUser.ScreenName),
				extractText(quotedTweet.FullText),
			)
		}
	}

	return caption
}

func (h *Handler) getFxTwitterData() *FxTwitterAPIData {
	request, response, err := utils.Request("https://api.fxtwitter.com/status/"+h.postID, utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})
	if err != nil || response.Body() == nil {
		return nil
	}
	defer utils.ReleaseRequestResources(request, response)

	var fxTwitterAPIData *FxTwitterAPIData
	err = json.Unmarshal(response.Body(), &fxTwitterAPIData)
	if err != nil {
		return nil
	}

	if fxTwitterAPIData == nil || fxTwitterAPIData.Code != 200 {
		return nil
	}

	return fxTwitterAPIData
}

func (h *Handler) processFxTwitterAPI(twitterData *FxTwitterAPIData) ([]telego.InputMedia, string) {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(twitterData.Tweet.Media.All)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range twitterData.Tweet.Media.All {
		go func(index int, twitterMedia FxTwitterMedia) {
			var media InputMedia
			var err error
			media.File, err = downloader.Downloader(twitterMedia.URL, fmt.Sprintf("SmudgeLord-Twitter_%d_%s_%s", index, h.username, h.postID))
			if err == nil && twitterMedia.Type == "video" {
				media.Thumbnail, _ = downloader.Downloader(twitterMedia.ThumbnailURL)
			}
			results <- mediaResult{index: index, media: &media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index,
				"Error", result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if twitterData.Tweet.Media.All[result.index].Type != "video" {
				mediaItem = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: result.media.File},
				}
			} else {
				mediaItem = &telego.InputMediaVideo{
					Type:              telego.MediaTypeVideo,
					Media:             telego.InputFile{File: result.media.File},
					Width:             twitterData.Tweet.Media.All[result.index].Width,
					Height:            twitterData.Tweet.Media.All[result.index].Height,
					SupportsStreaming: true,
				}
				if result.media.Thumbnail != nil {
					mediaItem.(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: result.media.Thumbnail}
					err := utils.ResizeThumbnail(result.media.Thumbnail)
					if err != nil {
						slog.Error("Failed to resize thumbnail",
							"Post Info", []string{h.username, h.postID},
							"Error", err.Error())
					}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	caption := fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n%s",
		twitterData.Tweet.Author.Name,
		twitterData.Tweet.Author.ScreenName,
		twitterData.Tweet.Text)

	return mediaItems, caption
}
