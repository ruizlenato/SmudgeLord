package instagram

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"smudgelord/smudgelord/modules/medias/downloader"
	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

func getEmbedData(post_id string) InstagramData {
	var instagramData InstagramData

	body := utils.Request(fmt.Sprintf("https://www.instagram.com/p/%v/embed/captioned/", post_id), utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"accept-language": "en-US,en;q=0.9",
			"connection":      "close",
			"sec-fetch-mode":  "navigate",
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
			"viewport-width":  "1280",
		},
	}).Body()
	if body == nil {
		return nil
	}

	if match := (regexp.MustCompile(`\\\"gql_data\\\":([\s\S]*)\}\"\}`)).FindSubmatch(body); len(match) == 2 {
		s := strings.ReplaceAll(string(match[1]), `\"`, `"`)
		s = strings.ReplaceAll(s, `\\/`, `/`)
		s = strings.ReplaceAll(s, `\\`, `\`)

		err := json.Unmarshal([]byte(s), &instagramData)
		if err != nil {
			log.Print("[instagram/getEmbed] Error unmarshalling Instagram data:", err)
		}
	}

	mediaTypeData := regexp.MustCompile(`(?s)data-media-type="(.*?)"`).FindAllStringSubmatch(string(body), -1)
	if instagramData == nil && len(mediaTypeData) > 0 && len(mediaTypeData[0]) > 1 && mediaTypeData[0][1] == "GraphImage" {
		// Get the main media
		re := regexp.MustCompile(`class="Content(.*?)src="(.*?)"`)
		mainMediaData := re.FindAllStringSubmatch(string(body), -1)
		mainMediaURL := (strings.ReplaceAll(mainMediaData[0][2], "amp;", ""))

		// Get the caption and owner
		var caption string
		var owner string
		re = regexp.MustCompile(`(?s)class="Caption"(.*?)class="CaptionUsername".*data-log-event="captionProfileClick" target="_blank">(.*?)<\/a>(.*?)<div`)
		captionData := re.FindAllStringSubmatch(string(body), -1)

		if len(captionData) > 0 && len(captionData[0]) > 2 {
			re = regexp.MustCompile(`<[^>]*>`)
			owner = strings.TrimSpace(re.ReplaceAllString(captionData[0][2], ""))
			caption = strings.TrimSpace(re.ReplaceAllString(captionData[0][3], ""))
		}

		dataJson := `{
			"shortcode_media": {
				"__typename": "GraphImage",
				"display_url": "` + mainMediaURL + `",
				"edge_media_to_caption": {
					"edges": [
						{
							"node": {
								"text": "` + caption + `"
							}
						}
					]
				},
				"owner": {
					"username": "` + owner + `"
				}
			}
			}`

		err := json.Unmarshal([]byte(dataJson), &instagramData)
		if err != nil {
			return nil
		}
	}

	return instagramData
}

// Fazer ele parser tudo e dps baixar, fazer somente uma chamada do downloader.
func getGQLData(post_id string) InstagramData {
	var instagramData InstagramData

	body := utils.Request("https://www.instagram.com/api/graphql", utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:         `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0`,
			`Accept`:             `*/*`,
			`Accept-Language`:    `en-US;q=0.5,en;q=0.3`,
			`Content-Type`:       `application/x-www-form-urlencoded`,
			`X-FB-Friendly-Name`: `PolarisPostActionLoadPostQueryQuery`,
			`X-CSRFToken`:        `-m5n6c-w1Z9RmrGqkoGTMq`,
			`X-IG-App-ID`:        `936619743392459`,
			`X-FB-LSD`:           `AVp2LurCmJw`,
			`X-ASBD-ID`:          `129477`,
			`DNT`:                `1`,
			`Sec-Fetch-Dest`:     `empty`,
			`Sec-Fetch-Mode`:     `cors`,
			`Sec-Fetch-Site`:     `same-origin`,
		},
		BodyString: []string{
			`av=0`,
			`__d=www`,
			`__user=0`,
			`__a=1`,
			`__req=3`,
			`__hs=19734.HYP:instagram_web_pkg.2.1..0.0`,
			`dpr=1`,
			`__ccg=UNKNOWN`,
			`__rev=1010782723`,
			`__s=qg5qgx:efei15:ng6310`,
			`__hsi=7323030086241513400`,
			`__dyn=7xeUjG1mxu1syUbFp60DU98nwgU29zEdEc8co2qwJw5ux609vCwjE1xoswIwuo2awlU-cw5Mx62G3i1ywOwv89k2C1Fwc60AEC7U2czXwae4UaEW2G1NwwwNwKwHw8Xxm16wUxO1px-0iS2S3qazo7u1xwIwbS1LwTwKG1pg661pwr86C1mwrd6goK68jxe6V8`,
			`__csr=gps8cIy8WTDAqjWDrpda9SoLHhaVeVEgvhaJzVQ8hF-qEPBV8O4EhGmciDBQh1mVuF9V9d2FHGicAVu8GAmfZiHzk9IxlhV94aKC5oOq6Uhx-Ku4Kaw04Jrx64-0oCdw0MXw1lm0EE2Ixcjg2Fg1JEko0N8U421tw62wq8989EMw1QpV60CE02BIw`,
			`__comet_req=7`,
			`lsd=AVp2LurCmJw`,
			`jazoest=2989`,
			`__spin_r=1010782723`,
			`__spin_b=trunk`,
			`__spin_t=1705025808`,
			`fb_api_caller_class=RelayModern`,
			`fb_api_req_friendly_name=PolarisPostActionLoadPostQueryQuery`,
			`query_hash=b3055c01b4b222b8a47dc12b090e4e64`,
			fmt.Sprintf(`variables={"shortcode": "%v","fetch_comment_count":2,"fetch_related_profile_media_count":0,"parent_comment_count":0,"child_comment_count":0,"fetch_like_count":10,"fetch_tagged_user_count":null,"fetch_preview_comment_count":2,"has_threaded_comments":true,"hoisted_comment_id":null,"hoisted_reply_id":null}`, post_id),
			`server_timestamps=true`,
			`doc_id=10015901848480474`,
		},
	}).Body()

	err := json.Unmarshal(body, &instagramData)
	if err != nil {
		log.Printf("[instagram/Instagram] Error unmarshalling Instagram data: %v", err)
		return nil
	}

	return instagramData
}

func Instagram(url string) ([]telego.InputMedia, string) {
	var mu sync.Mutex
	var mediaItems []telego.InputMedia
	var caption string
	var postID string

	if regexp.MustCompile(`(?:stories)/`).MatchString(url) {
		var storiesData StoriesData
		body := utils.Request("https://scrapper.ruizlenato.workers.dev/"+url, utils.RequestParams{
			Method: "GET",
		}).Body()
		if body == nil {
			return nil, caption
		}
		if err := json.Unmarshal(body, &storiesData); err != nil {
			log.Printf("[instagram/Instagram] Error unmarshalling instagram stories data: %v", err)
			return nil, caption
		}

		file, err := downloader.Downloader(storiesData.URL)
		if err != nil {
			log.Print("[instagram/Instagram] Error downloading file:", err)
			defer file.Close()
			return nil, caption
		}

		if strings.Contains(storiesData.URL, ".mp4?") {
			mediaItems = append(mediaItems, telegoutil.MediaVideo(telegoutil.File(file)))
		}
		if strings.Contains(storiesData.URL, ".jpg?") || strings.Contains(storiesData.URL, ".png?") {
			mediaItems = append(mediaItems, telegoutil.MediaPhoto(telegoutil.File(file)))
		}
		return mediaItems, caption
	}

	if matches := regexp.MustCompile(`(?:reel(?:s?)|p)/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 2 {
		postID = matches[1]
	} else {
		return nil, caption
	}

	var instagramData *ShortcodeMedia
	if data := getEmbedData(postID); data != nil && data.ShortcodeMedia != nil {
		instagramData = data.ShortcodeMedia
	} else if data := getGQLData(postID); data != nil && data.Data.XDTShortcodeMedia != nil {
		instagramData = data.Data.XDTShortcodeMedia
	}

	if instagramData == nil {
		return nil, caption
	}

	if len(instagramData.EdgeMediaToCaption.Edges) > 0 {
		var sb strings.Builder

		if username := instagramData.Owner.Username; username != "" {
			sb.WriteString(fmt.Sprintf("<b>%v</b>", username))
		}

		if coauthors := instagramData.CoauthorProducers; coauthors != nil && len(*coauthors) > 0 {
			for i, coauthor := range *coauthors {
				if i > 0 {
					sb.WriteString(" <b>&</b> ")
				}
				sb.WriteString(fmt.Sprintf("<b>%v</b>", coauthor.Username))
			}
		}

		if sb.Len() > 0 {
			sb.WriteString("<b>:</b>\n")
		}
		sb.WriteString(instagramData.EdgeMediaToCaption.Edges[0].Node.Text)

		caption = sb.String()
	}

	switch instagramData.Typename {
	case "GraphVideo", "XDTGraphVideo":
		file, err := downloader.Downloader(instagramData.VideoURL)
		thumbnail, _ := downloader.Downloader(instagramData.DisplayResources[len(instagramData.DisplayResources)-1].Src)
		if err != nil {
			log.Print("[instagram/Instagram] Error downloading video:", err)
			return nil, caption
		}
		mediaItems = append(mediaItems, &telego.InputMediaVideo{
			Type:              telego.MediaTypeVideo,
			Media:             telego.InputFile{File: file},
			Thumbnail:         &telego.InputFile{File: thumbnail},
			Width:             instagramData.Dimensions.Width,
			Height:            instagramData.Dimensions.Height,
			SupportsStreaming: true,
		})
	case "GraphImage", "XDTGraphImage":
		file, err := downloader.Downloader(instagramData.DisplayURL)
		if err != nil {
			log.Print("[instagram/Instagram] Error downloading image:", err)
			return nil, caption
		}
		mediaItems = append(mediaItems, telegoutil.MediaPhoto(
			telegoutil.File(file)),
		)
	case "GraphSidecar", "XDTGraphSidecar":
		var wg sync.WaitGroup

		type InputMedia struct {
			File      *os.File
			Thumbnail *os.File
		}
		medias := make(map[int]*InputMedia)

		for i, results := range instagramData.EdgeSidecarToChildren.Edges {
			wg.Add(1)
			go func(index int, result Edges) {
				defer wg.Done()
				mu.Lock()
				defer mu.Unlock()

				var media InputMedia
				var err error

				if !result.Node.IsVideo {
					media.File, err = downloader.Downloader(result.Node.DisplayResources[len(result.Node.DisplayResources)-1].Src)
				} else {
					media.File, err = downloader.Downloader(result.Node.VideoURL)
					if err == nil {
						media.Thumbnail, _ = downloader.Downloader(result.Node.DisplayResources[len(result.Node.DisplayResources)-1].Src)
					}
				}
				if err != nil {
					log.Print("[instagram/Instagram] Error downloading media:", err)
					// Use index as key to store nil for failed downloads
					medias[index] = &InputMedia{File: nil, Thumbnail: nil}
					return
				}
				// Use index as key to store downloaded file
				medias[index] = &media
			}(i, results)
		}

		wg.Wait()
		mediaItems = make([]telego.InputMedia, len(medias))

		// Process results after all downloads are complete
		for index, media := range medias {
			if media.File != nil {
				if !instagramData.EdgeSidecarToChildren.Edges[index].Node.IsVideo {
					mediaItems[index] = telegoutil.MediaPhoto(telegoutil.File(media.File))
				} else {
					if media.Thumbnail != nil {
						mediaItems[index] = &telego.InputMediaVideo{
							Type:              telego.MediaTypeVideo,
							Media:             telego.InputFile{File: media.File},
							Thumbnail:         &telego.InputFile{File: media.Thumbnail},
							Width:             instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources[len(instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources)-1].ConfigWidth,
							Height:            instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources[len(instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources)-1].ConfigHeight,
							SupportsStreaming: true,
						}
					} else {
						// Proceed without thumbnail
						mediaItems[index] = &telego.InputMediaVideo{
							Type:              telego.MediaTypeVideo,
							Media:             telego.InputFile{File: media.File},
							Width:             instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources[len(instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources)-1].ConfigWidth,
							Height:            instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources[len(instagramData.EdgeSidecarToChildren.Edges[index].Node.DisplayResources)-1].ConfigHeight,
							SupportsStreaming: true,
						}
					}
				}
			}
		}
	}

	return mediaItems, caption
}
