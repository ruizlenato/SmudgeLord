package xiaohongshu

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	username string
	postID   string
}

func Handle(message *telegram.NewMessage) ([]telegram.InputMedia, []string) {
	handler := &Handler{}
	postURL := handler.getPostURL(message.Text())
	if postURL == "" {
		return nil, []string{}
	}
	handler.extractPostID(postURL)

	if cachedMedias, cachedCaption, err := downloader.GetMediaCache(handler.postID); err == nil {
		return cachedMedias, []string{cachedCaption, handler.postID}
	}

	xiaohongshuData := handler.getPostData(postURL)
	if xiaohongshuData == nil {
		return nil, []string{}
	}

	noteData, noteDataExists := xiaohongshuData.Note.NoteDetailMap[handler.postID]
	if !noteDataExists {
		return nil, []string{}
	}

	switch noteData.Note.Type {
	case "video":
		return handler.downloadVideo(noteData, message), []string{getCaption(noteData), handler.postID}
	case "normal":
		return handler.downloadImages(noteData, message), []string{getCaption(noteData), handler.postID}
	default:
		return nil, []string{}
	}
}

func (h *Handler) extractPostID(url string) {
	postIDRegex := regexp.MustCompile(`/(?:explore|item)/(\w+)`)
	if matches := postIDRegex.FindStringSubmatch(url); len(matches) > 1 {
		h.postID = matches[1]
	}
}

func (h *Handler) getPostURL(text string) string {
	if strings.Contains(text, "explore/") {
		return text
	}

	if strings.Contains(text, "xhslink") {
		retryCaller := &utils.RetryCaller{
			Caller:       utils.DefaultHTTPCaller,
			MaxAttempts:  3,
			ExponentBase: 2,
			StartDelay:   1 * time.Second,
			MaxDelay:     5 * time.Second,
		}

		response, err := retryCaller.Request(text, utils.RequestParams{
			Headers:   downloader.GenericHeaders,
			Method:    "GET",
			Redirects: 2,
		})

		if err != nil {
			return ""
		}
		defer response.Body.Close()

		text = response.Request.URL.String()
		parsedURL, err := url.Parse(text)
		if err != nil {
			slog.Error(
				"Error parsing URL",
				"Error", err.Error(),
			)
			return ""
		}
		if parsedURL.Query().Has("redirectPath") {
			text = parsedURL.Query().Get("redirectPath")
		}
	}

	return strings.Replace(text, "/discovery/item/", "/explore/", 1)
}

var (
	scriptRegex = regexp.MustCompile(`(?m)<script>window.__INITIAL_STATE__=(.+?)</script>`)
)

func (h *Handler) getPostData(url string) XiaohongshuData {
	var xiaohongshuData XiaohongshuData

	response, err := utils.Request(url, utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})

	if err != nil {
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

	if matches := scriptRegex.FindSubmatch(body); len(matches) > 1 {
		xiaohongshuJson := strings.ReplaceAll(string(matches[1]), "undefined", "null")
		err := json.Unmarshal([]byte(xiaohongshuJson), &xiaohongshuData)
		if err != nil {
			slog.Error(
				"Error unmarshalling JSON to struct",
				"Post Info", []string{h.username, h.postID},
				"Error", err.Error(),
			)
			return nil
		}

	}

	return xiaohongshuData
}

func getCaption(noteData Note) string {
	caption := fmt.Sprintf("<b>%s</b>:", noteData.Note.User.Nickname)
	if noteData.Note.Title != "" {
		caption += fmt.Sprintf("\n<b>%s</b>", noteData.Note.Title)
	}
	if noteData.Note.Desc != "" {
		caption += fmt.Sprintf("\n%s", noteData.Note.Desc)
	}

	return caption
}

func (h *Handler) downloadVideo(noteData Note, message *telegram.NewMessage) []telegram.InputMedia {
	videoInfo := h.findFirstAvailableVideoFormat(noteData.Note.Video.Media.Stream)
	if videoInfo == nil {
		slog.Error(
			"No valid video format found",
			"Post Info", []string{h.username, h.postID},
		)
		return nil
	}

	file, err := downloader.FetchBytesFromURL(videoInfo.MasterURL)
	if err != nil {
		slog.Error(
			"Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return nil
	}

	video, err := helpers.UploadVideo(message, helpers.UploadVideoParams{
		File:              file,
		SupportsStreaming: true,
		Width:             int32(videoInfo.Width),
		Height:            int32(videoInfo.Height),
	})
	if err != nil {
		slog.Error(
			"Failed to upload video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error(),
		)
		return nil
	}

	return []telegram.InputMedia{&video}
}

// following the priority: AV1 > H266 > H265 > H264
func (h *Handler) findFirstAvailableVideoFormat(stream VideoStream) VideoInfo {
	formats := [][]VideoInfo{
		stream.Av1,
		stream.H266,
		stream.H265,
		stream.H264,
	}

	for _, format := range formats {
		if len(format) > 0 {
			return format[0]
		}
	}
	return nil
}

func (h *Handler) downloadImages(noteData Note, message *telegram.NewMessage) []telegram.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
	}

	mediaCount := len(noteData.Note.ImageList)
	mediaItems := make([]telegram.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range noteData.Note.ImageList {
		go func(index int, media Images) {
			url := media.URLDefault
			if media.LivePhoto {
				videoInfo := h.findFirstAvailableVideoFormat(media.Stream)
				url = videoInfo.MasterURL
			}

			file, err := downloader.FetchBytesFromURL(url)
			if err != nil {
				slog.Error(
					"Failed to download image",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error(),
				)

			}
			results <- mediaResult{index, file}
		}(i, media)
	}

	for range mediaCount {
		result := <-results
		if result.file != nil {
			if noteData.Note.ImageList[result.index].LivePhoto {
				video, err := helpers.UploadVideo(message, helpers.UploadVideoParams{
					File:              result.file,
					SupportsStreaming: true,
				})
				if err != nil {
					slog.Error(
						"Failed to upload video",
						"Post Info", []string{h.username, h.postID},
						"Error", err.Error(),
					)
				}
				mediaItems[result.index] = &video
			} else {
				photo, err := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
					File: result.file,
				})
				if err != nil {
					slog.Error(
						"Failed to upload photo",
						"Post Info", []string{h.username, h.postID},
						"Error", err.Error(),
					)
				}
				mediaItems[result.index] = &photo
			}
		}
	}

	return mediaItems
}
