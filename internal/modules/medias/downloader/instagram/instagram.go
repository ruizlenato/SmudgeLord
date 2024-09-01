package instagram

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) ([]telego.InputMedia, []string) {
	var medias []telego.InputMedia

	postID := getPostID(text)
	if postID == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, postID}
	}

	instagramData := getInstagramData(postID)
	if instagramData == nil {
		return nil, []string{}
	}

	caption := getCaption(instagramData)

	switch instagramData.Typename {
	case "GraphVideo", "XDTGraphVideo":
		medias = handleVideo(instagramData)
	case "GraphImage", "XDTGraphImage":
		medias = handleImage(instagramData)
	case "GraphSidecar", "XDTGraphSidecar":
		medias = handleSidecar(instagramData)
	default:
		return nil, []string{}
	}

	return medias, []string{caption, postID}
}

func getPostID(url string) (postID string) {
	if matches := regexp.MustCompile(`(?:reel(?:s?)|p)/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 2 {
		return matches[1]
	}
	return postID
}

func getInstagramData(postID string) *ShortcodeMedia {
	if data := getEmbedData(postID); data != nil && data.ShortcodeMedia != nil {
		return data.ShortcodeMedia
	} else if data := getScrapperAPIData(postID); data != nil && data.ShortcodeMedia != nil {
		return data.ShortcodeMedia
	} else if data := getGQLData(postID); data != nil && data.Data.XDTShortcodeMedia != nil {
		return data.Data.XDTShortcodeMedia
	}

	return nil
}

func getCaption(instagramData *ShortcodeMedia) string {
	if len(instagramData.EdgeMediaToCaption.Edges) > 0 {
		var sb strings.Builder

		if username := instagramData.Owner.Username; username != "" {
			sb.WriteString(fmt.Sprintf("<b>%v</b>", username))
		}

		if coauthors := instagramData.CoauthorProducers; coauthors != nil && len(*coauthors) > 0 {
			if sb.Len() > 0 {
				sb.WriteString(" <b>&</b> ")
			}
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

		return sb.String()
	}
	return ""
}

var (
	mediaTypeRegex = regexp.MustCompile(`(?s)data-media-type="(.*?)"`)
	mainMediaRegex = regexp.MustCompile(`class="Content(.*?)src="(.*?)"`)
	captionRegex   = regexp.MustCompile(`(?s)class="Caption"(.*?)class="CaptionUsername".*data-log-event="captionProfileClick" target="_blank">(.*?)<\/a>(.*?)<div`)
	htmlTagRegex   = regexp.MustCompile(`<[^>]*>`)
)

func getEmbedData(postID string) InstagramData {
	var instagramData InstagramData

	body := utils.Request(fmt.Sprintf("https://www.instagram.com/p/%v/embed/captioned/", postID), utils.RequestParams{
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
			log.Print("[instagram/getEmbed] Error unmarshalling Instagram data: ", err)
		}
	}

	mediaTypeData := mediaTypeRegex.FindAllStringSubmatch(string(body), -1)
	if instagramData == nil && len(mediaTypeData) > 0 && len(mediaTypeData[0]) > 1 && mediaTypeData[0][1] == "GraphImage" {
		mainMediaData := mainMediaRegex.FindAllStringSubmatch(string(body), -1)
		mainMediaURL := (strings.ReplaceAll(mainMediaData[0][2], "amp;", ""))

		var caption string
		var owner string
		captionData := captionRegex.FindAllStringSubmatch(string(body), -1)

		if len(captionData) > 0 && len(captionData[0]) > 2 {
			owner = strings.TrimSpace(htmlTagRegex.ReplaceAllString(captionData[0][2], ""))
			caption = strings.TrimSpace(htmlTagRegex.ReplaceAllString(captionData[0][3], ""))
		}

		dataJSON := `{
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

		err := json.Unmarshal([]byte(dataJSON), &instagramData)
		if err != nil {
			return nil
		}
	}

	return instagramData
}

func getScrapperAPIData(postID string) InstagramData {
	var instagramData InstagramData

	body := utils.Request("https://scrapper.ruizlenato.tech/instagram", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"id": postID,
		},
	}).Body()

	err := json.Unmarshal(body, &instagramData)
	if err != nil {
		log.Print("Instagram: Error unmarshalling ScrapperAPIdata: ", err)
		return nil
	}

	return instagramData
}

func getGQLData(postID string) InstagramData {
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
			fmt.Sprintf(`variables={"shortcode": "%v","fetch_comment_count":2,"fetch_related_profile_media_count":0,"parent_comment_count":0,"child_comment_count":0,"fetch_like_count":10,"fetch_tagged_user_count":null,"fetch_preview_comment_count":2,"has_threaded_comments":true,"hoisted_comment_id":null,"hoisted_reply_id":null}`, postID),
			`server_timestamps=true`,
			`doc_id=10015901848480474`,
		},
	}).Body()

	err := json.Unmarshal(body, &instagramData)
	if err != nil {
		log.Print("Instagram: Error unmarshalling GQLData: ", err)
		return nil
	}

	return instagramData
}

func handleVideo(instagramData *ShortcodeMedia) []telego.InputMedia {
	file, err := downloader.Downloader(instagramData.VideoURL)
	if err != nil {
		log.Print("Instagram: Error downloading video: ", err)
		return nil
	}

	thumbnail, err := downloader.Downloader(instagramData.DisplayResources[len(instagramData.DisplayResources)-1].Src)
	if err != nil {
		log.Print("Instagram: Error downloading thumbnail: ", err)
		return nil
	}
	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: file},
		Thumbnail:         &telego.InputFile{File: thumbnail},
		Width:             instagramData.Dimensions.Width,
		Height:            instagramData.Dimensions.Height,
		SupportsStreaming: true,
	}}
}

func handleImage(instagramData *ShortcodeMedia) []telego.InputMedia {
	file, err := downloader.Downloader(instagramData.DisplayURL)
	if err != nil {
		log.Print("Instagram: Error downloading image:", err)
		return nil
	}

	return []telego.InputMedia{&telego.InputMediaPhoto{
		Type:  telego.MediaTypePhoto,
		Media: telego.InputFile{File: file},
	}}
}

type InputMedia struct {
	File      *os.File
	Thumbnail *os.File
}

func handleSidecar(instagramData *ShortcodeMedia) []telego.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(instagramData.EdgeSidecarToChildren.Edges)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, result := range instagramData.EdgeSidecarToChildren.Edges {
		go func(index int, result Edges) {
			var media InputMedia
			var err error
			if !result.Node.IsVideo {
				media.File, err = downloader.Downloader(result.Node.DisplayResources[len(result.Node.DisplayResources)-1].Src)
			} else {
				media.File, err = downloader.Downloader(result.Node.VideoURL)
				if err == nil {
					media.Thumbnail, err = downloader.Downloader(result.Node.DisplayResources[len(result.Node.DisplayResources)-1].Src)
				}
			}
			results <- mediaResult{index: index, media: &media, err: err}
		}(i, result)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			log.Print(result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if !instagramData.EdgeSidecarToChildren.Edges[result.index].Node.IsVideo {
				mediaItem = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: result.media.File},
				}
			} else {
				mediaItem = &telego.InputMediaVideo{
					Type:              telego.MediaTypeVideo,
					Media:             telego.InputFile{File: result.media.File},
					Width:             instagramData.Dimensions.Width,
					Height:            instagramData.Dimensions.Height,
					SupportsStreaming: true,
				}
				if result.media.Thumbnail != nil {
					mediaItem.(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: result.media.Thumbnail}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}
