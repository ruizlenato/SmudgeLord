package twitter

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"sync"

	"smudgelord/smudgelord/modules/medias/downloader"
	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

var headers = map[string]string{
	"Authorization":             "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
	"x-twitter-client-language": "en",
	"x-twitter-active-user":     "yes",
	"Accept-language":           "en",
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	"content-type":              "application/json",
}

func getGuestToken() string {
	type guestToken struct {
		GuestToken string `json:"guest_token"`
	}

	body := utils.RequestPOST("https://api.twitter.com/1.1/guest/activate.json", utils.RequestPOSTParams{Headers: headers}).Body()
	var res guestToken
	err := json.Unmarshal(body, &res)
	if err != nil {
		log.Printf("Error unmarshalling guest token: %v", err)
	}
	return res.GuestToken
}

func TweetExtract(tweetID string) *TwitterAPIData {
	headers["x-guest-token"] = getGuestToken()
	headers["cookie"] = fmt.Sprintf("guest_id=v1:%v;", getGuestToken())
	variables := map[string]interface{}{
		"tweetId":                                tweetID,
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

	query := map[string]string{
		"variables": string(jsonMarshal(variables)),
		"features":  string(jsonMarshal(features)),
	}

	body := utils.RequestGET("https://twitter.com/i/api/graphql/5GOHgZe-8U2j5sVHQzEm9A/TweetResultByRestId", utils.RequestGETParams{Query: query, Headers: headers}).Body()
	if body == nil {
		return nil
	}
	var twitterAPIData *TwitterAPIData
	err := json.Unmarshal(body, &twitterAPIData)
	if err != nil {
		log.Printf("Error unmarshalling Twitter data: %v", err)
		return nil
	}

	return twitterAPIData
}

func Twitter(url string) ([]telego.InputMedia, string) {
	var mediaItems []telego.InputMedia
	var caption string
	var tweetID string

	if matches := regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`).FindStringSubmatch(url); len(matches) == 2 {
		tweetID = matches[1]
	} else {
		return nil, ""
	}

	twitterAPIData := TweetExtract(tweetID)

	if twitterAPIData == nil || (*twitterAPIData).Data.TweetResults.Legacy == nil {
		return nil, ""
	}

	var wg sync.WaitGroup
	wg.Add(len((*twitterAPIData).Data.TweetResults.Result.Legacy.ExtendedEntities.Media))
	mediaItems = make([]telego.InputMedia, len((*twitterAPIData).Data.TweetResults.Result.Legacy.ExtendedEntities.Media))

	medias := make(map[int]*os.File)

	for i, media := range (*twitterAPIData).Data.TweetResults.Result.Legacy.ExtendedEntities.Media {
		go func(index int, media Media) {
			defer wg.Done()
			var file *os.File
			var err error
			var videoType string

			if slices.Contains([]string{"animated_gif", "video"}, media.Type) {
				videoType = "video"
			}
			if videoType != "video" {
				file, err = downloader.Downloader(media.MediaURLHTTPS)
			} else {
				sort.Slice(media.VideoInfo.Variants, func(i, j int) bool {
					return media.VideoInfo.Variants[i].Bitrate < media.VideoInfo.Variants[j].Bitrate
				})
				file, err = downloader.Downloader(media.VideoInfo.Variants[len(media.VideoInfo.Variants)-1].URL)
			}
			if err != nil {
				log.Print("[twitter/Twitter] Error downloading media:", err)
				// Use index as key to store nil for failed downloads
				medias[index] = nil
				return
			}
			// Use index as key to store downloaded file
			medias[index] = file
		}(i, media)
	}

	wg.Wait()

	// Process medias after all downloads are complete
	for index, file := range medias {
		if file != nil {
			var mediaItem telego.InputMedia
			if (*twitterAPIData).Data.TweetResults.Result.Legacy.ExtendedEntities.Media[index].Type == "photo" {
				mediaItem = telegoutil.MediaPhoto(telegoutil.File(file))
			} else {
				mediaItem = telegoutil.MediaVideo(telegoutil.File(file)).WithWidth(((*twitterAPIData).Data.TweetResults.Result.Legacy.ExtendedEntities.Media)[index].OriginalInfo.Width).WithHeight(((*twitterAPIData).Data.TweetResults.Result.Legacy.ExtendedEntities.Media)[index].OriginalInfo.Height)
			}
			mediaItems[index] = mediaItem
		}
	}

	if tweet := (*twitterAPIData).Data.TweetResults.Result.Legacy; tweet != nil {
		caption = tweet.FullText
	}

	return mediaItems, caption
}
