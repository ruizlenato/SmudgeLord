package instagram

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	username string
	postID   string
}

func Handle(text string) ([]telego.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return nil, []string{}
	}

	medias, caption, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return medias, []string{caption, handler.postID}
	}

	data := handler.getInstagramData()
	if data == nil {
		slog.Error("Failed to fetch Instagram data",
			"Post Info", []string{handler.username, handler.postID})
		return nil, []string{}
	}

	handler.username = data.Owner.Username
	return handler.processMedia(data), []string{getCaption(data), handler.postID}
}

func (h *Handler) setPostID(url string) bool {
	postIDRegex := regexp.MustCompile(`(?:reel(?:s?)|p)/([A-Za-z0-9_-]+)`)

	if matches := postIDRegex.FindStringSubmatch(url); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultFastHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	request, response, err := retryCaller.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return false
	}

	if matches := postIDRegex.FindStringSubmatch(request.URI().String()); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	return false
}

func (h *Handler) getInstagramData() *ShortcodeMedia {
	for _, fetchFunc := range []func() InstagramData{
		h.getEmbedData, h.getScrapperAPIData, h.getGQLData,
	} {
		if data := fetchFunc(); data != nil {
			if data.ShortcodeMedia == nil && data.Data.XDTShortcodeMedia != nil {
				return data.Data.XDTShortcodeMedia
			}
			return data.ShortcodeMedia
		}
	}
	return nil
}

func (h *Handler) processMedia(data *ShortcodeMedia) []telego.InputMedia {
	switch data.Typename {
	case "GraphVideo", "XDTGraphVideo":
		return h.handleVideo(data)
	case "GraphImage", "XDTGraphImage":
		return h.handleImage(data)
	case "GraphSidecar", "XDTGraphSidecar":
		return h.handleSidecar(data)
	default:
		return nil
	}
}

func getCaption(data *ShortcodeMedia) string {
	if len(data.EdgeMediaToCaption.Edges) == 0 {
		return ""
	}

	var sb strings.Builder
	if data.Owner.Username != "" {
		fmt.Fprintf(&sb, "<b>%s</b>", data.Owner.Username)
	}

	if coauthors := data.CoauthorProducers; coauthors != nil {
		for i, coauthor := range *coauthors {
			if i > 0 {
				sb.WriteString(" <b>&</b> ")
			}
			fmt.Fprintf(&sb, "<b>%s</b>", coauthor.Username)
		}
	}

	if sb.Len() > 0 {
		sb.WriteString("<b>:</b>\n")
	}
	sb.WriteString(data.EdgeMediaToCaption.Edges[0].Node.Text)

	return sb.String()
}

var (
	mediaTypeRegex = regexp.MustCompile(`(?s)data-media-type="(.*?)"`)
	mainMediaRegex = regexp.MustCompile(`class="Content(.*?)src="(.*?)"`)
	captionRegex   = regexp.MustCompile(`(?s)class="Caption"(.*?)class="CaptionUsername".*data-log-event="captionProfileClick" target="_blank">(.*?)<\/a>(.*?)<div`)
	htmlTagRegex   = regexp.MustCompile(`<[^>]*>`)
)

func (h *Handler) getEmbedData() InstagramData {
	var data InstagramData

	request, response, err := utils.Request(fmt.Sprintf("https://www.instagram.com/p/%v/embed/captioned/", h.postID), utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"accept-language": "en-US,en;q=0.9",
			"connection":      "close",
			"sec-fetch-mode":  "navigate",
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
			"viewport-width":  "1280",
		},
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response.Body() == nil {
		return nil
	}

	if match := (regexp.MustCompile(`\\\"gql_data\\\":([\s\S]*)\}\"\}`)).FindSubmatch(response.Body()); len(match) == 2 {
		s := strings.ReplaceAll(string(match[1]), `\"`, `"`)
		s = strings.ReplaceAll(s, `\\/`, `/`)
		s = strings.ReplaceAll(s, `\\`, `\`)

		json.Unmarshal([]byte(s), &data)
	}

	mediaTypeData := mediaTypeRegex.FindAllStringSubmatch(string(response.Body()), -1)
	if data == nil && len(mediaTypeData) > 0 && len(mediaTypeData[0]) > 1 && mediaTypeData[0][1] == "GraphImage" {
		mainMediaData := mainMediaRegex.FindAllStringSubmatch(string(response.Body()), -1)
		mainMediaURL := (strings.ReplaceAll(mainMediaData[0][2], "amp;", ""))

		var caption string
		var owner string
		captionData := captionRegex.FindAllStringSubmatch(string(response.Body()), -1)

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

		err := json.Unmarshal([]byte(dataJSON), &data)
		if err != nil {
			return nil
		}
	}

	return data
}

func (h *Handler) getScrapperAPIData() InstagramData {
	var data InstagramData

	request, response, err := utils.Request("https://scrapper.ruizlenato.tech/instagram", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"id": h.postID,
		},
	})
	defer utils.ReleaseRequestResources(request, response)
	if err != nil || response.Body() == nil {
		return nil
	}

	err = json.Unmarshal(response.Body(), &data)
	if err != nil {
		return nil
	}

	return data
}

func (h *Handler) getGQLData() InstagramData {
	var data InstagramData

	request, response, err := utils.Request("https://www.instagram.com/graphql/query", utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:      `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0`,
			`Accept-Language`: `en-US;q=0.5,en;q=0.3`,
			`Content-Type`:    `application/x-www-form-urlencoded`,
			`X-CSRFToken`:     `JKA19cNYckTn_Dr6bcTO5F`,
			`X-IG-App-ID`:     `936619743392459`,
			`X-FB-LSD`:        `AVqBX1zadbA`,
			`Sec-Fetch-Site`:  `same-origin`,
		},
		BodyString: []string{
			`lsd=AVqBX1zadbA`,
			`jazoest=2949`,
			fmt.Sprintf(`variables={"shortcode": "%v","fetch_comment_count":0,"fetch_related_profile_media_count":0,"parent_comment_count":null}`, h.postID),
			`doc_id=8845758582119845`,
		},
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response.Body() == nil {
		return nil
	}

	err = json.Unmarshal(response.Body(), &data)
	if err != nil {
		return nil
	}

	return data
}

func (h *Handler) handleVideo(data *ShortcodeMedia) []telego.InputMedia {
	file, err := downloader.Downloader(data.VideoURL)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.Downloader(data.DisplayResources[len(data.DisplayResources)-1].Src)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err)
		return nil
	}

	err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err)
	}

	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: file},
		Thumbnail:         &telego.InputFile{File: thumbnail},
		Width:             data.Dimensions.Width,
		Height:            data.Dimensions.Height,
		SupportsStreaming: true,
	}}
}

func (h *Handler) handleImage(data *ShortcodeMedia) []telego.InputMedia {
	file, err := downloader.Downloader(data.DisplayURL)
	if err != nil {
		slog.Error("Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	return []telego.InputMedia{&telego.InputMediaPhoto{
		Type:  telego.MediaTypePhoto,
		Media: telego.InputFile{File: file},
	}}
}

func (h *Handler) handleSidecar(data *ShortcodeMedia) []telego.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(data.EdgeSidecarToChildren.Edges)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range data.EdgeSidecarToChildren.Edges {
		go func(index int, edge Edges) {
			media, err := h.downloadMedia(index, edge)
			results <- mediaResult{index: index, media: media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in sidecar",
				"Post Info", []string{h.username, h.postID},
				"Error", result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if !data.EdgeSidecarToChildren.Edges[result.index].Node.IsVideo {
				mediaItem = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: result.media.File},
				}
			} else {
				mediaItem = &telego.InputMediaVideo{
					Type:              telego.MediaTypeVideo,
					Media:             telego.InputFile{File: result.media.File},
					Width:             data.Dimensions.Width,
					Height:            data.Dimensions.Height,
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

func (h *Handler) downloadMedia(index int, data Edges) (*InputMedia, error) {
	var media InputMedia
	var err error
	filename := fmt.Sprintf("SmudgeLord-Instagram_%d_%s_%s", index, h.username, h.postID)

	if !data.Node.IsVideo {
		media.File, err = downloader.Downloader(data.Node.DisplayResources[len(data.Node.DisplayResources)-1].Src, filename)
	} else {
		media.File, err = downloader.Downloader(data.Node.VideoURL, filename)
		if err == nil {
			media.Thumbnail, err = downloader.Downloader(data.Node.DisplayResources[len(data.Node.DisplayResources)-1].Src)
		}
	}
	if err != nil {
		return nil, err
	}
	return &media, nil
}
