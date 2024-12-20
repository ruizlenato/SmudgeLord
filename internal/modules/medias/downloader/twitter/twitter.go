package twitter

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	twitterAPIURL = "https://twitter.com/i/api/graphql/5GOHgZe-8U2j5sVHQzEm9A/TweetResultByRestId"
	guestTokenURL = "https://api.twitter.com/1.1/guest/activate.json"
)

var headers = map[string]string{
	"Authorization":             "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
	"x-twitter-client-language": "en",
	"x-twitter-active-user":     "yes",
	"Accept-language":           "en",
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	"content-type":              "application/json",
}

func Handle(text string) ([]telego.InputMedia, []string) {
	postID := getPostID(text)
	if postID == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, postID}
	}

	twitterData := getTwitterData(postID)
	if twitterData == nil {
		return nil, []string{}
	}

	if (*twitterData).Data.TweetResult.Result.Reason != nil && *(*twitterData).Data.TweetResult.Result.Reason == "NsfwLoggedOut" {
		fxTwitterData := getFxTwitterData(postID)
		if fxTwitterData == nil {
			return nil, []string{}
		}
		medias, caption := processFxTwitterAPI(fxTwitterData)
		return medias, []string{caption, postID}
	}

	medias := processTwitterAPI(twitterData)
	caption := getCaption(twitterData)
	return medias, []string{caption, postID}
}

func getPostID(url string) (postID string) {
	if matches := regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`).FindStringSubmatch(url); len(matches) == 2 {
		return matches[1]
	} else {
		return postID
	}
}

type InputMedia struct {
	File      *os.File
	Thumbnail *os.File
}

func processTwitterAPI(twitterData *TwitterAPIData) []telego.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len((*twitterData).Data.TweetResult.Result.Legacy.ExtendedEntities.Media)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range (*twitterData).Data.TweetResult.Result.Legacy.ExtendedEntities.Media {
		go func(index int, twitterMedia Media) {
			media, err := downloadMedia(twitterMedia)
			results <- mediaResult{index: index, media: media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			log.Print(result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if (*twitterData).Data.TweetResult.Result.Legacy.ExtendedEntities.Media[result.index].Type == "photo" {
				mediaItem = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: result.media.File},
				}
			} else {
				mediaItem = &telego.InputMediaVideo{
					Type:              telego.MediaTypeVideo,
					Media:             telego.InputFile{File: result.media.File},
					Width:             ((*twitterData).Data.TweetResult.Result.Legacy.ExtendedEntities.Media)[result.index].OriginalInfo.Width,
					Height:            ((*twitterData).Data.TweetResult.Result.Legacy.ExtendedEntities.Media)[result.index].OriginalInfo.Height,
					SupportsStreaming: true,
				}
				if result.media.Thumbnail != nil {
					err := utils.ResizeThumbnail(result.media.Thumbnail)
					if err != nil {
						log.Print("Twitter — Error resizing thumbnail: " + err.Error())
					}
					mediaItem.(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: result.media.Thumbnail}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func downloadMedia(twitterMedia Media) (*InputMedia, error) {
	var media InputMedia
	var err error

	if slices.Contains([]string{"animated_gif", "video"}, twitterMedia.Type) {
		sort.Slice(twitterMedia.VideoInfo.Variants, func(i, j int) bool {
			return twitterMedia.VideoInfo.Variants[i].Bitrate < twitterMedia.VideoInfo.Variants[j].Bitrate
		})
		media.File, err = downloader.Downloader(twitterMedia.VideoInfo.Variants[len(twitterMedia.VideoInfo.Variants)-1].URL)
		if err == nil {
			media.Thumbnail, _ = downloader.Downloader(twitterMedia.MediaURLHTTPS)
		}
	} else {
		media.File, err = downloader.Downloader(twitterMedia.MediaURLHTTPS)
	}

	if err != nil {
		return nil, err
	}
	return &media, nil
}

func getGuestToken() string {
	type guestToken struct {
		GuestToken string `json:"guest_token"`
	}
	var res guestToken

	headers = map[string]string{
		"Authorization": "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
		"User-Agent":    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	}

	request, response, err := utils.Request(guestTokenURL, utils.RequestParams{
		Method:    "POST",
		Headers:   headers,
		Redirects: 3,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		log.Print("Error getting guest token: ", err)
		return ""
	}

	err = json.Unmarshal(response.Body(), &res)
	if err != nil {
		log.Printf("Error unmarshalling guest token: %v", err)
		return ""
	}
	return res.GuestToken
}

func getTwitterData(postID string) *TwitterAPIData {
	guestToken := getGuestToken()
	if guestToken == "" {
		return nil
	}

	headers["x-guest-token"] = guestToken
	headers["cookie"] = fmt.Sprintf("guest_id=v1:%v;", guestToken)

	variables := map[string]interface{}{
		"tweetId":                                postID,
		"referrer":                               "messages",
		"includePromotedContent":                 true,
		"withCommunity":                          true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withBirdwatchNotes":                     true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
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

	jsonMarshal := func(data interface{}) []byte {
		result, _ := json.Marshal(data)
		return result
	}

	request, response, err := utils.Request(twitterAPIURL, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"variables": string(jsonMarshal(variables)),
			"features":  string(jsonMarshal(features)),
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
		(*twitterAPIData).Data.TweetResult.Result.Reason != nil &&
		*(*twitterAPIData).Data.TweetResult.Result.Reason == "NsfwLoggedOut" {
		return twitterAPIData
	}

	if twitterAPIData == nil ||
		(*twitterAPIData).Data.TweetResult == nil ||
		(*twitterAPIData).Data.TweetResult.Legacy == nil {
		return nil
	}

	return twitterAPIData
}

func getCaption(twitterData *TwitterAPIData) string {
	var caption string

	if tweet := (*twitterData).Data.TweetResult.Result.Legacy; tweet != nil {
		caption = fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n",
			(*twitterData).Data.TweetResult.Result.Core.UserResults.Result.Legacy.Name,
			(*twitterData).Data.TweetResult.Result.Core.UserResults.Result.Legacy.ScreenName)

		if idx := strings.LastIndex(tweet.FullText, " https://t.co/"); idx != -1 {
			caption += tweet.FullText[:idx]
		}
	}

	return caption
}

func getFxTwitterData(postID string) *FxTwitterAPIData {
	request, response, err := utils.Request("https://api.fxtwitter.com/status/"+postID, utils.RequestParams{
		Method:  "GET",
		Headers: headers,
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

func processFxTwitterAPI(twitterData *FxTwitterAPIData) ([]telego.InputMedia, string) {
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
			media.File, err = downloader.Downloader(twitterMedia.URL)
			if err == nil && twitterMedia.Type == "video" {
				media.Thumbnail, _ = downloader.Downloader(twitterMedia.ThumbnailURL)
			}
			results <- mediaResult{index: index, media: &media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			log.Print(result.err)
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
						log.Print("Twitter — Error resizing thumbnail: " + err.Error())
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
