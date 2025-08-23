package twitter

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	twitterAPIURL   = "https://twitter.com/i/api/graphql/2ICDjqPd81tulZcYrtpTuQ/TweetResultByRestId"
	guestTokenURL   = "https://api.twitter.com/1.1/guest/activate.json"
	fxTwitterAPIURL = "https://api.fxtwitter.com/status/"
)

var headers = map[string]string{
	"Authorization":             "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
	"x-twitter-client-language": "en",
	"x-twitter-active-user":     "yes",
	"content-type":              "application/json",
}

type Handler struct {
	username string
	postID   string
}

func Handle(message string) ([]telegram.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setPostID(message) {
		return nil, []string{}
	}

	if cachedMedias, cachedCaption, err := downloader.GetMediaCache(handler.postID); err == nil {
		return cachedMedias, []string{cachedCaption, handler.postID}
	}

	twitterData, err := handler.getTwitterData()
	if err != nil || twitterData == nil ||
		(*twitterData).Data.TweetResult == nil || (*twitterData).Data.TweetResult.Legacy == nil ||
		(*twitterData).Data.TweetResult.Result.Reason != nil && *(*twitterData).Data.TweetResult.Result.Reason == "NsfwLoggedOut" {
		fxTwitterData := handler.getFxTwitterData()
		if fxTwitterData == nil {
			return nil, []string{}
		}
		medias, caption := handler.processFxTwitterAPI(fxTwitterData)
		return medias, []string{caption, handler.postID}
	}

	medias, caption := handler.processTwitterAPI(twitterData)
	return medias, []string{caption, handler.postID}
}

func (h *Handler) setPostID(url string) bool {
	if matches := regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`).FindStringSubmatch(url); len(matches) == 2 {
		h.postID = matches[1]
		return true
	}

	return false
}

func (h *Handler) processTwitterAPI(twitterData *TwitterAPIData) ([]telegram.InputMedia, string) {
	type mediaResult struct {
		index int
		media *InputMedia
	}

	allTweetMedia := (*twitterData).Data.TweetResult.Legacy.ExtendedEntities.Media
	mediaCount := len(allTweetMedia)
	if mediaCount > 10 {
		mediaCount = 10
		allTweetMedia = allTweetMedia[:10]
	}

	mediaChan := make(chan mediaResult, mediaCount)
	mediaItems := make([]telegram.InputMedia, mediaCount)

	for i, media := range allTweetMedia {
		go func(index int, twitterMedia Media) {
			var media InputMedia
			var err error

			if slices.Contains([]string{"animated_gif", "video"}, twitterMedia.Type) {
				sort.Slice(twitterMedia.VideoInfo.Variants, func(i, j int) bool {
					return twitterMedia.VideoInfo.Variants[i].Bitrate < twitterMedia.VideoInfo.Variants[j].Bitrate
				})

				media.File, err = downloader.FetchBytesFromURL(twitterMedia.VideoInfo.Variants[len(twitterMedia.VideoInfo.Variants)-1].URL)
				if err != nil {
					slog.Error(
						"Failed to download video",
						"Post Info", []string{h.username, h.postID},
						"Media URL", twitterMedia.VideoInfo.Variants[len(twitterMedia.VideoInfo.Variants)-1].URL,
						"Error", err.Error(),
					)
					mediaChan <- mediaResult{index, nil}
					return
				}

				media.Thumbnail, err = downloader.FetchBytesFromURL(twitterMedia.MediaURLHTTPS)
				if err != nil {
					slog.Error(
						"Failed to download thumbnail",
						"Post Info", []string{h.username, h.postID},
						"Thumbnail URL", twitterMedia.MediaURLHTTPS,
						"Error", err.Error(),
					)
					mediaChan <- mediaResult{index, nil}
					return
				}

				media.Thumbnail, err = utils.ResizeThumbnailFromBytes(media.Thumbnail)
				if err != nil {
					slog.Error(
						"Failed to resize thumbnail",
						"Post Info", []string{h.username, h.postID},
						"Thumbnail URL", twitterMedia.MediaURLHTTPS,
						"Error", err.Error(),
					)
				}

			} else {
				media.File, err = downloader.FetchBytesFromURL(twitterMedia.MediaURLHTTPS)
				if err != nil {
					slog.Error(
						"Failed to download image",
						"Post Info", []string{h.username, h.postID},
						"Image URL", twitterMedia.MediaURLHTTPS,
						"Error", err.Error(),
					)
					return
				}
			}

			mediaChan <- mediaResult{index, &media}
		}(i, media)
	}

	for range mediaCount {
		result := <-mediaChan
		if result.media.File != nil {
			twitterMedia := allTweetMedia[result.index]
			if twitterMedia.Type == "photo" {
				photo, err := helpers.UploadPhoto(helpers.UploadPhotoParams{
					File: result.media.File,
				})
				if err != nil {
					if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
						slog.Error(
							"Failed to upload photo",
							"Post Info", []string{h.username, h.postID},
							"Photo URL", twitterMedia.MediaURLHTTPS,
							"Error", err.Error(),
						)
					}
					continue
				}

				mediaItems[result.index] = &photo
			} else {
				video, err := helpers.UploadVideo(helpers.UploadVideoParams{
					File:              result.media.File,
					Thumb:             result.media.Thumbnail,
					SupportsStreaming: true,
					Width:             int32(twitterMedia.OriginalInfo.Width),
					Height:            int32(twitterMedia.OriginalInfo.Height),
				})
				if err != nil {
					if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
						slog.Error(
							"Failed to upload video",
							"Post Info", []string{h.username, h.postID},
							"Video URL", twitterMedia.MediaURLHTTPS,
							"Error", err.Error(),
						)
					}
					continue
				}

				mediaItems[result.index] = &video
			}
		}
	}

	caption := getCaption(twitterData)
	return mediaItems, caption
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
		Headers:   headers,
		Redirects: 3,
	})

	if err != nil {
		slog.Error(
			"Failed to get guest token",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return ""
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil {
		slog.Error(
			"Failed to unmarshal guest token",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return ""
	}

	cache.SetCache("twitter_guest_token", res.GuestToken, 3*time.Hour)
	return res.GuestToken
}

func (h *Handler) getTwitterData() (*TwitterAPIData, error) {
	guestToken := h.getGuestToken()
	if guestToken == "" {
		return nil, errors.New("failed to get guest token")
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
		return nil, errors.New("response body is nil")
	}
	defer response.Body.Close()

	var twitterAPIData *TwitterAPIData
	err = json.NewDecoder(response.Body).Decode(&twitterAPIData)
	if err != nil {
		return nil, err
	}

	return twitterAPIData, nil
}

func getCaption(twitterData *TwitterAPIData) string {
	var caption string

	if tweet := (*twitterData).Data.TweetResult.Result.Legacy; tweet != nil {
		caption = (fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n",
			(*twitterData).Data.TweetResult.Result.Core.UserResults.Result.Legacy.Name,
			(*twitterData).Data.TweetResult.Result.Core.UserResults.Result.Legacy.ScreenName))

		if idx := strings.LastIndex(tweet.FullText, " https://t.co/"); idx != -1 {
			caption += (tweet.FullText[:idx])
		}
	}

	return caption
}

func (h *Handler) getFxTwitterData() *FxTwitterAPIData {
	response, err := utils.Request(fxTwitterAPIURL+h.postID, utils.RequestParams{
		Method:  "GET",
		Headers: headers,
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

func (h *Handler) processFxTwitterAPI(twitterData *FxTwitterAPIData) ([]telegram.InputMedia, string) {
	type mediaResult struct {
		index int
		media *InputMedia
	}

	mediaCount := len(twitterData.Tweet.Media.All)
	mediaItems := make([]telegram.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range twitterData.Tweet.Media.All {
		go func(index int, twitterMedia FxTwitterMedia) {
			var media InputMedia
			var err error
			media.File, err = downloader.FetchBytesFromURL(twitterMedia.URL)
			if err == nil && twitterMedia.Type == "video" {
				media.Thumbnail, err = downloader.FetchBytesFromURL(twitterMedia.ThumbnailURL)
				if err != nil {
					slog.Error(
						"Failed to download media",
						"Post Info", []string{h.username, h.postID},
						"Media URL", twitterMedia.URL,
						"Error", err.Error(),
					)
				}

				media.Thumbnail, err = utils.ResizeThumbnailFromBytes(media.Thumbnail)
				if err != nil {
					slog.Error(
						"Failed to resize thumbnail",
						"Post Info", []string{h.username, h.postID},
						"Media URL", twitterMedia.URL,
						"Error", err.Error(),
					)
				}
			}
			if err != nil {
				slog.Error(
					"Failed to download media",
					"Post Info", []string{h.username, h.postID},
					"Media URL", twitterMedia.URL,
					"Error", err.Error(),
				)
			}
			results <- mediaResult{index: index, media: &media}
		}(i, media)
	}

	for range mediaCount {
		result := <-results
		if result.media.File != nil {
			var mediaItem telegram.InputMedia
			if !slices.Contains([]string{"gif", "video"}, twitterData.Tweet.Media.All[result.index].Type) {
				uploadedPhoto, err := helpers.UploadPhoto(helpers.UploadPhotoParams{
					File: result.media.File,
				})
				if err != nil {
					if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
						slog.Error(
							"Failed to upload photo",
							"Post Info", []string{h.username, h.postID},
							"Photo URL", twitterData.Tweet.Media.All[result.index].URL,
							"Error", err.Error(),
						)
					}
					continue
				}
				mediaItem = &uploadedPhoto
			} else {
				uploadedVideo, err := helpers.UploadVideo(helpers.UploadVideoParams{
					File:              result.media.File,
					Thumb:             result.media.Thumbnail,
					SupportsStreaming: true,
					Width:             int32(twitterData.Tweet.Media.All[result.index].Width),
					Height:            int32(twitterData.Tweet.Media.All[result.index].Height),
				})
				if err != nil {
					if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
						slog.Error(
							"Failed to upload video",
							"Post Info", []string{h.username, h.postID},
							"Video URL", twitterData.Tweet.Media.All[result.index].URL,
							"Error", err.Error(),
						)
					}
					continue
				}
				mediaItem = &uploadedVideo
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
