package medias

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"slices"
	"sort"
	"strings"

	"smudgelord/smudgelord/utils"

	"github.com/google/uuid"
	"github.com/mymmrac/telego/telegoutil"
)

type TwitterAPIData struct {
	Data *struct {
		ThreadedConversationWithInjectionsV2 *struct {
			Instructions []struct {
				Entries []struct {
					EntryID string `json:"entryId"`
					Content struct {
						ItemContent struct {
							TweetResults struct {
								Result `json:"result"`
							} `json:"tweet_results"`
						} `json:"itemContent"`
					} `json:"content"`
				} `json:"entries,omitempty"`
			} `json:"instructions"`
		} `json:"threaded_conversation_with_injections_v2"`
		User *struct {
			Result struct {
				Legacy Legacy `json:"legacy"`
			} `json:"result"`
		} `json:"user,omitempty"`
	} `json:"data"`
}

type Result struct {
	Typename string `json:"__typename"`
	Tweet    struct {
		Legacy Legacy `json:"legacy"`
	} `json:"tweet"`
	Legacy Legacy `json:"legacy"`
}

type Legacy *struct {
	FullText         string `json:"full_text"`
	ExtendedEntities struct {
		Media []struct {
			DisplayURL           string `json:"display_url"`
			ExpandedURL          string `json:"expanded_url"`
			Indices              []int  `json:"indices"`
			MediaURLHTTPS        string `json:"media_url_https"`
			Type                 string `json:"type"`
			URL                  string `json:"url"`
			ExtMediaAvailability struct {
				Status string `json:"status"`
			} `json:"ext_media_availability"`
			Sizes struct {
				Large size `json:"large"`
				Thumb size `json:"thumb"`
			} `json:"sizes"`
			OriginalInfo struct {
				Height     int   `json:"height"`
				Width      int   `json:"width"`
				FocusRects []any `json:"focus_rects"`
			} `json:"original_info"`
			VideoInfo struct {
				AspectRatio    []int `json:"aspect_ratio"`
				DurationMillis int   `json:"duration_millis"`
				Variants       []struct {
					Bitrate     int    `json:"bitrate,omitempty"`
					ContentType string `json:"content_type"`
					URL         string `json:"url"`
				} `json:"variants"`
			} `json:"video_info"`
		} `json:"media"`
	} `json:"extended_entities"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Entities    struct {
		Description struct {
			Urls []interface{} `json:"urls"`
		} `json:"description"`
	} `json:"entities"`
	Verified       bool   `json:"verified"`
	FollowersCount int    `json:"followers_count"`
	FriendsCount   int    `json:"friends_count"`
	StatusesCount  int    `json:"statuses_count"`
	Location       string `json:"location"`
}

type size struct {
	H      int    `json:"h"`
	W      int    `json:"w"`
	Resize string `json:"resize"`
}

func TweetExtract(tweetID string) *TwitterAPIData {
	csrfToken := strings.ReplaceAll(uuid.New().String(), "-", "")
	headers := map[string]string{
		"Authorization":             "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
		"Cookie":                    fmt.Sprintf("auth_token=ee4ebd1070835b90a9b8016d1e6c6130ccc89637; ct0=%v; ", csrfToken),
		"x-twitter-active-user":     "yes",
		"x-twitter-auth-type":       "OAuth2Session",
		"x-twitter-client-language": "en",
		"x-csrf-token":              csrfToken,
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	}
	variables := map[string]interface{}{
		"focalTweetId":                           tweetID,
		"referrer":                               "messages",
		"includePromotedContent":                 true,
		"withCommunity":                          true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withBirdwatchNotes":                     true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	features := map[string]interface{}{
		"rweb_lists_timeline_redesign_enabled":                                    true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"tweetypie_unmention_optimization_enabled":                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              false,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                false,
		"tweet_awards_web_tipping_enabled":                                        false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_media_download_video_enabled":                             false,
		"responsive_web_enhance_cards_enabled":                                    false,
	}
	fieldtoggles := map[string]interface{}{
		"withAuxiliaryUserLabels":     false,
		"withArticleRichContentState": false,
	}

	jsonMarshal := func(data interface{}) []byte {
		result, _ := json.Marshal(data)
		return result
	}

	query := map[string]string{
		"variables":    string(jsonMarshal(variables)),
		"features":     string(jsonMarshal(features)),
		"fieldToggles": string(jsonMarshal(fieldtoggles)),
	}

	body := utils.RequestGET("https://twitter.com/i/api/graphql/NmCeCgkVlsRGS1cAwqtgmw/TweetDetail", utils.RequestGETParams{Query: query, Headers: headers}).Body()
	var twitterAPIData *TwitterAPIData
	err := json.Unmarshal(body, &twitterAPIData)
	if err != nil {
		log.Printf("[twitter/TweetExtract] Error unmarshalling Twitter data: %v", err)
		return nil
	}

	return twitterAPIData
}

func (dm *DownloadMedia) Twitter(url string) {
	var tweetID string

	if matches := regexp.MustCompile(`.*(?:twitter|x).com/.+status/([A-Za-z0-9]+)`).FindStringSubmatch(url); len(matches) == 2 {
		tweetID = matches[1]
	} else {
		return
	}

	twitterAPIData := TweetExtract(tweetID)

	var tweetResult interface{}
	if twitterAPIData.Data.ThreadedConversationWithInjectionsV2 == nil {
		return
	}
	for _, entry := range twitterAPIData.Data.ThreadedConversationWithInjectionsV2.Instructions[0].Entries {
		if entry.EntryID == fmt.Sprintf("tweet-%v", tweetID) {
			if entry.Content.ItemContent.TweetResults.Result.Typename == "TweetWithVisibilityResults" {
				tweetResult = entry.Content.ItemContent.TweetResults.Result.Tweet.Legacy
			} else {
				tweetResult = entry.Content.ItemContent.TweetResults.Result.Legacy
			}
			break
		}
	}
	if tweetResult.(Legacy) == nil {
		return
	}

	for _, media := range tweetResult.(Legacy).ExtendedEntities.Media {
		var videoType string
		if slices.Contains([]string{"animated_gif", "video"}, media.Type) {
			videoType = "video"
		}
		if videoType != "video" {
			file, err := downloader(media.MediaURLHTTPS)
			if err != nil {
				log.Print("[twitter/Twitter] Error downloading photo:", err)
				return
			}
			dm.MediaItems = append(dm.MediaItems, telegoutil.MediaPhoto(
				telegoutil.File(file)),
			)
		} else {
			sort.Slice(media.VideoInfo.Variants, func(i, j int) bool {
				return media.VideoInfo.Variants[i].Bitrate < media.VideoInfo.Variants[j].Bitrate
			})
			file, err := downloader(media.VideoInfo.Variants[len(media.VideoInfo.Variants)-1].URL)
			if err != nil {
				log.Print("[twitter/Twitter] Error downloading video:", err)
				return
			}
			dm.MediaItems = append(dm.MediaItems, telegoutil.MediaVideo(
				telegoutil.File(file)).WithWidth(media.OriginalInfo.Width).WithHeight(media.OriginalInfo.Height),
			)
		}
		if tweet, ok := tweetResult.(Legacy); ok {
			dm.Caption = tweet.FullText
		}
	}
}
