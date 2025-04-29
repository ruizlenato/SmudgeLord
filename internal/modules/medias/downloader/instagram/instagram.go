package instagram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	username string
	postID   string
}

func Handle(text string) ([]models.InputMedia, []string) {
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
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	response, err := retryCaller.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})

	if err != nil {
		return false
	}
	defer response.Body.Close()

	if matches := postIDRegex.FindStringSubmatch(response.Request.URL.User.String()); len(matches) > 1 {
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
			if data.Status == "ok" && data.ShortcodeMedia == nil && data.Data.XDTShortcodeMedia == nil {
				slog.Error("Failed to fetch Instagram data",
					"Post Info", []string{h.username, h.postID})
			}

			if data.ShortcodeMedia == nil && data.Data.XDTShortcodeMedia != nil {
				return data.Data.XDTShortcodeMedia
			}
			return data.ShortcodeMedia
		}
	}
	return nil
}

func (h *Handler) processMedia(data *ShortcodeMedia) []models.InputMedia {
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

func getCaption(instagramData *ShortcodeMedia) string {
	if len(instagramData.EdgeMediaToCaption.Edges) > 0 {
		var sb strings.Builder

		if username := instagramData.Owner.Username; username != "" {
			sb.WriteString(fmt.Sprintf("<a href='instagram.com/%v'><b>%v</b></a>", username, username))
		}

		if coauthors := instagramData.CoauthorProducers; coauthors != nil && len(*coauthors) > 0 {
			if sb.Len() > 0 {
				sb.WriteString(" <b>&</b> ")
			}
			for i, coauthor := range *coauthors {
				if i > 0 {
					sb.WriteString(" <b>&</b> ")
				}

				sb.WriteString(fmt.Sprintf("<a href='instagram.com/%v'><b>%v</b></a>", coauthor.Username, coauthor.Username))
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

func (h *Handler) getEmbedData() InstagramData {
	var data InstagramData

	response, err := utils.Request(fmt.Sprintf("https://www.instagram.com/p/%v/embed/captioned/", h.postID), utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Error("Failed to read response body",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	if match := (regexp.MustCompile(`\\\"gql_data\\\":([\s\S]*)\}\"\}`)).FindSubmatch(body); len(match) == 2 {
		s := strings.ReplaceAll(string(match[1]), `\"`, `"`)
		s = strings.ReplaceAll(s, `\\/`, `/`)
		s = strings.ReplaceAll(s, `\\`, `\`)

		json.Unmarshal([]byte(s), &data)
	}

	mediaTypeData := regexp.MustCompile(`(?s)data-media-type="(.*?)"`).FindAllStringSubmatch(string(body), -1)
	if data == nil && len(mediaTypeData) > 0 && len(mediaTypeData[0]) > 1 && mediaTypeData[0][1] == "GraphImage" {
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

		err := json.Unmarshal([]byte(dataJSON), &data)
		if err != nil {
			return nil
		}
	}

	return data
}

func (h *Handler) getScrapperAPIData() InstagramData {
	var data InstagramData

	response, err := utils.Request("https://scrapper.ruizlenato.tech/instagram", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"id": h.postID,
		},
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return nil
	}

	return data
}

func (h *Handler) getGQLData() InstagramData {
	var data InstagramData

	downloader.GenericHeaders["Content-Type"] = "application/x-www-form-urlencoded"
	downloader.GenericHeaders["X-CSRFToken"] = "JKA19cNYckTn_Dr6bcTO5F"
	downloader.GenericHeaders["X-IG-App-ID"] = "936619743392459"
	downloader.GenericHeaders["X-FB-LSD"] = "AVqBX1zadbA"
	downloader.GenericHeaders["Sec-Fetch-Site"] = "same-origin"
	response, err := utils.Request("https://www.instagram.com/graphql/query", utils.RequestParams{
		Method:  "POST",
		Headers: downloader.GenericHeaders,
		BodyString: []string{
			fmt.Sprintf(`variables={"shortcode": "%v","fetch_comment_count":0,"fetch_related_profile_media_count":0,"parent_comment_count":null}`, h.postID),
			`doc_id=8845758582119845`,
		},
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return nil
	}

	return data
}

func (h *Handler) handleVideo(data *ShortcodeMedia) []models.InputMedia {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Instagram_%s_%s", h.username, h.postID))
	file, err := downloader.FetchBytesFromURL(data.VideoURL)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(data.DisplayResources[len(data.DisplayResources)-1].Src)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err)
		return nil
	}

	thumbnail, err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}

	return []models.InputMedia{&models.InputMediaVideo{
		Media: "attach://" + filename,
		Thumbnail: &models.InputFileUpload{
			Filename: filename,
			Data:     bytes.NewBuffer(thumbnail),
		},
		Width:             data.Dimensions.Width,
		Height:            data.Dimensions.Height,
		SupportsStreaming: true,
		MediaAttachment:   bytes.NewBuffer(file),
	}}
}

func (h *Handler) handleImage(data *ShortcodeMedia) []models.InputMedia {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Instagram_%s_%s", h.username, h.postID))
	file, err := downloader.FetchBytesFromURL(data.DisplayURL)
	if err != nil {
		slog.Error("Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	return []models.InputMedia{&models.InputMediaPhoto{
		Media:           "attach://" + filename,
		MediaAttachment: bytes.NewBuffer(file),
	}}
}

func (h *Handler) handleSidecar(data *ShortcodeMedia) []models.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(data.EdgeSidecarToChildren.Edges)
	mediaItems := make([]models.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range data.EdgeSidecarToChildren.Edges {
		go func(index int, edge Edges) {
			media, err := h.downloadMedia(edge)
			results <- mediaResult{index: index, media: media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in sidecar",
				"Post Info", []string{h.username, h.postID},
				"Error", result.err.Error())
			continue
		}
		if result.media.File != nil {
			filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Instagram_%d_%s_%s", result.index, h.username, h.postID))
			var mediaItem models.InputMedia
			if !data.EdgeSidecarToChildren.Edges[result.index].Node.IsVideo {
				mediaItem = &models.InputMediaPhoto{
					Media:           "attach://" + filename,
					MediaAttachment: bytes.NewBuffer(result.media.File),
				}
			} else {
				mediaItem = &models.InputMediaVideo{
					Media:             "attach://" + filename,
					Width:             data.Dimensions.Width,
					Height:            data.Dimensions.Height,
					SupportsStreaming: true,
					MediaAttachment:   bytes.NewBuffer(result.media.File),
				}
				if result.media.Thumbnail != nil {
					mediaItem.(*models.InputMediaVideo).Thumbnail = &models.InputFileUpload{
						Filename: filename,
						Data:     bytes.NewBuffer(result.media.Thumbnail),
					}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func (h *Handler) downloadMedia(data Edges) (*InputMedia, error) {
	var media InputMedia
	var err error

	if !data.Node.IsVideo {
		media.File, err = downloader.FetchBytesFromURL(data.Node.DisplayResources[len(data.Node.DisplayResources)-1].Src)
	} else {
		media.File, err = downloader.FetchBytesFromURL(data.Node.VideoURL)
		if err == nil {
			media.Thumbnail, err = downloader.FetchBytesFromURL(data.Node.DisplayResources[len(data.Node.DisplayResources)-1].Src)
		}
	}
	if err != nil {
		return nil, err
	}
	return &media, nil
}
