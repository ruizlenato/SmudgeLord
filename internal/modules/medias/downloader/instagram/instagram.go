package instagram

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
)

func Handle(message string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(message) {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(handler.postID); err == nil {
		return postInfo
	}

	data := handler.getInstagramData()
	if data == nil {
		return downloader.PostInfo{}
	}

	return downloader.PostInfo{
		ID:      handler.postID,
		Medias:  handler.processMedia(data),
		Caption: getCaption(data),
	}
}

func (h *Handler) setPostID(url string) bool {
	shareRegex := regexp.MustCompile(`(?:instagram.com)/(?:share)`)
	if shareRegex.MatchString(url) {
		retryCaller := &utils.RetryCaller{
			Caller:       utils.DefaultHTTPCaller,
			MaxAttempts:  3,
			ExponentBase: 2,
			StartDelay:   1 * time.Second,
			MaxDelay:     5 * time.Second,
		}

		response, err := retryCaller.Request(url, utils.RequestParams{
			Method:    "GET",
			Redirects: 4,
		})

		if err != nil {
			return false
		}
		defer response.Body.Close()

		url = response.Request.URL.String()
	}
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

func (h *Handler) processMedia(instagramData *ShortcodeMedia) []telegram.InputMedia {
	switch instagramData.Typename {
	case "GraphVideo", "XDTGraphVideo":
		return h.handleVideo(instagramData)
	case "GraphImage", "XDTGraphImage":
		return h.handleImage(instagramData)
	case "GraphSidecar", "XDTGraphSidecar":
		return h.handleSidecar(instagramData)
	default:
		return nil
	}
}

func (h *Handler) getInstagramData() *ShortcodeMedia {
	for _, fetchFunc := range []func() InstagramData{
		h.getEmbedData, h.getScrapperAPIData, h.getGQLData,
	} {
		if data := fetchFunc(); data != nil {
			if data.Status == "ok" && data.ShortcodeMedia == nil && data.Data.XDTShortcodeMedia == nil {
				slog.Error(
					"Failed to fetch Instagram data",
					"Post Info", []string{h.username, h.postID},
				)
			}

			if data.ShortcodeMedia == nil && data.Data.XDTShortcodeMedia != nil {
				return data.Data.XDTShortcodeMedia
			}
			return data.ShortcodeMedia
		}
	}
	return nil
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
			sb.WriteString("\n")
		}
		sb.WriteString(instagramData.EdgeMediaToCaption.Edges[0].Node.Text)

		return sb.String()
	}
	return ""
}

func (h *Handler) getEmbedData() InstagramData {
	var instagramData InstagramData

	response, err := utils.Request(fmt.Sprintf("https://www.instagram.com/p/%v/embed/captioned/", h.postID), utils.RequestParams{
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

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Error(
			"Failed to read response body",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return nil
	}

	if match := (regexp.MustCompile(`\\\"gql_data\\\":([\s\S]*)\}\"\}`)).FindSubmatch(body); len(match) == 2 {
		s := strings.ReplaceAll(string(match[1]), `\"`, `"`)
		s = strings.ReplaceAll(s, `\\/`, `/`)
		s = strings.ReplaceAll(s, `\\`, `\`)

		err := json.Unmarshal([]byte(s), &instagramData)
		if err != nil {
			slog.Error(
				"Failed to unmarshal Instagram data",
				"Post Info", []string{h.username, h.postID},
				"Error", err.Error(),
			)
			return nil
		}
	}

	mediaTypeData := regexp.MustCompile(`(?s)data-media-type="(.*?)"`).FindAllStringSubmatch(string(body), -1)
	if instagramData == nil && len(mediaTypeData) > 0 && len(mediaTypeData[0]) > 1 && mediaTypeData[0][1] == "GraphImage" {
		re := regexp.MustCompile(`class="Content(.*?)src="(.*?)"`)
		mainMediaData := re.FindAllStringSubmatch(string(body), -1)
		mainMediaURL := (strings.ReplaceAll(mainMediaData[0][2], "amp;", ""))

		var caption string
		var owner string
		re = regexp.MustCompile(`(?s)class="Caption"(.*?)class="CaptionUsername".*data-log-event="captionProfileClick" target="_blank">(.*?)<\/a>(.*?)<div`)
		captionData := re.FindAllStringSubmatch(string(body), -1)

		if len(captionData) > 0 && len(captionData[0]) > 2 {
			re = regexp.MustCompile(`<[^>]*>`)
			captionText := strings.ReplaceAll(captionData[0][3], "<br />", "\n")
			owner = strings.TrimSpace(re.ReplaceAllString(captionData[0][2], ""))
			caption = strings.TrimSpace(re.ReplaceAllString(captionText, ""))
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
		return instagramData
	}

	return nil
}

func (h *Handler) getScrapperAPIData() InstagramData {
	var data InstagramData

	response, err := utils.Request("https://scrapper.ruizlenato.loseyourip.com/instagram", utils.RequestParams{
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
func (h *Handler) handleVideo(instagramData *ShortcodeMedia) []telegram.InputMedia {
	file, err := downloader.FetchBytesFromURL(instagramData.VideoURL)
	if err != nil {
		slog.Error(
			"Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(instagramData.DisplayResources[len(instagramData.DisplayResources)-1].Src)
	if err != nil {
		slog.Error(
			"Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return nil
	}

	thumbnail, err = utils.ResizeThumbnailFromBytes(thumbnail)
	if err != nil {
		slog.Error(
			"Failed to resize thumbnail",
			"Thumbnail URL", instagramData.DisplayResources[len(instagramData.DisplayResources)-1].Src,
			"Error", err.Error(),
		)
	}

	video, err := helpers.UploadVideo(helpers.UploadVideoParams{
		File:              file,
		Thumb:             thumbnail,
		SupportsStreaming: true,
		Width:             int32(instagramData.Dimensions.Width),
		Height:            int32(instagramData.Dimensions.Height),
	})
	if err != nil {
		if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
			slog.Error(
				"Failed to upload video",
				"Post Info", []string{h.username, h.postID},
				"Error", err.Error(),
			)
		}
		return nil
	}

	return []telegram.InputMedia{&video}
}

func (h *Handler) handleImage(instagramData *ShortcodeMedia) []telegram.InputMedia {
	file, err := downloader.FetchBytesFromURL(instagramData.DisplayURL)
	if err != nil {
		slog.Error(
			"Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return nil
	}

	photo, err := helpers.UploadPhoto(helpers.UploadPhotoParams{
		File: file,
	})
	if err != nil {
		if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
			slog.Error(
				"Failed to upload photo",
				"Post Info", []string{h.username, h.postID},
				"Error", err.Error(),
			)
		}
		return nil
	}

	return []telegram.InputMedia{&photo}
}

func (h *Handler) handleSidecar(instagramData *ShortcodeMedia) []telegram.InputMedia {
	edges := h.prepareSidecarEdges(instagramData.EdgeSidecarToChildren.Edges)
	mediaCount := len(edges)

	mediaItems := make([]telegram.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, edge := range edges {
		go h.processSidecarMedia(i, edge, results)
	}

	for range mediaCount {
		result := <-results
		if result.file != nil {
			uploadedMedia := h.uploadSidecarMedia(result, edges[result.index], instagramData)
			if uploadedMedia != nil {
				mediaItems[result.index] = uploadedMedia
			}
		}
	}

	return mediaItems
}

func (h *Handler) prepareSidecarEdges(edges []Edges) []Edges {
	mediaCount := len(edges)
	if mediaCount > 10 {
		return edges[:10]
	}
	return edges
}

func (h *Handler) processSidecarMedia(index int, edge Edges, results chan<- mediaResult) {
	media := h.downloadSidecarMediaFiles(edge)
	results <- mediaResult{index, media}
}

func (h *Handler) downloadSidecarMediaFiles(edge Edges) *InputMedia {
	var media InputMedia
	var err error

	if !edge.Node.IsVideo {
		media.File, err = h.downloadSidecarImage(edge)
	} else {
		media.File, err = h.downloadSidecarVideo(edge)
		if err == nil {
			media.Thumbnail = h.downloadAndResizeThumbnail(edge)
		}
	}

	if err != nil {
		return nil
	}
	return &media
}

func (h *Handler) downloadSidecarImage(edge Edges) ([]byte, error) {
	imageURL := edge.Node.DisplayResources[len(edge.Node.DisplayResources)-1].Src
	file, err := downloader.FetchBytesFromURL(imageURL)
	if err != nil {
		slog.Error(
			"Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Image URL", imageURL,
			"Error", err.Error(),
		)
	}
	return file, err
}

func (h *Handler) downloadSidecarVideo(edge Edges) ([]byte, error) {
	file, err := downloader.FetchBytesFromURL(edge.Node.VideoURL)
	if err != nil {
		slog.Error(
			"Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Video URL", edge.Node.VideoURL,
			"Error", err.Error(),
		)
	}
	return file, err
}

func (h *Handler) downloadAndResizeThumbnail(edge Edges) []byte {
	thumbnailURL := edge.Node.DisplayResources[len(edge.Node.DisplayResources)-1].Src
	thumbnail, err := downloader.FetchBytesFromURL(thumbnailURL)
	if err != nil {
		slog.Error(
			"Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error(),
		)
		return nil
	}

	resizedThumbnail, err := utils.ResizeThumbnailFromBytes(thumbnail)
	if err != nil {
		slog.Error(
			"Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error(),
		)
		return thumbnail
	}
	return resizedThumbnail
}

func (h *Handler) uploadSidecarMedia(result mediaResult, edge Edges, instagramData *ShortcodeMedia) telegram.InputMedia {
	if !edge.Node.IsVideo {
		return h.uploadSidecarPhoto(result)
	}
	return h.uploadSidecarVideo(result, instagramData)
}

func (h *Handler) uploadSidecarPhoto(result mediaResult) telegram.InputMedia {
	photo, err := helpers.UploadPhoto(helpers.UploadPhotoParams{
		File: result.file.File,
	})
	if err != nil {
		if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
			slog.Error(
				"Failed to upload photo",
				"Post Info", []string{h.username, h.postID},
				"Error", err.Error(),
			)
		}
		return nil
	}
	return &photo
}

func (h *Handler) uploadSidecarVideo(result mediaResult, instagramData *ShortcodeMedia) telegram.InputMedia {
	video, err := helpers.UploadVideo(helpers.UploadVideoParams{
		File:              result.file.File,
		Thumb:             result.file.Thumbnail,
		SupportsStreaming: true,
		Width:             int32(instagramData.Dimensions.Width),
		Height:            int32(instagramData.Dimensions.Height),
	})
	if err != nil {
		if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
			slog.Error(
				"Failed to upload video",
				"Post Info", []string{h.username, h.postID},
				"Error", err.Error(),
			)
		}
		return nil
	}
	return &video
}
