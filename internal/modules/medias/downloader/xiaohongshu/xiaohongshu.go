package xiaohongshu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
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
	postURL := handler.getPostURL(text)
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
		return handler.downloadVideo(noteData), []string{getCaption(noteData), handler.postID}
	case "normal":
		return handler.downloadImages(noteData), []string{getCaption(noteData), handler.postID}
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
			slog.Error("Error parsing URL",
				"Error", err)
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
		slog.Error("Failed to read response body",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	if matches := scriptRegex.FindSubmatch(body); len(matches) > 1 {
		xiaohongshuJson := strings.ReplaceAll(string(matches[1]), "undefined", "null")
		err := json.Unmarshal([]byte(xiaohongshuJson), &xiaohongshuData)
		if err != nil {
			slog.Error("Error unmarshalling JSON to struct",
				"Error", err)
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

func (h *Handler) downloadVideo(noteData Note) []models.InputMedia {
	videoInfo := h.findFirstAvailableVideoFormat(noteData.Note.Video.Media.Stream)
	if videoInfo == nil {
		slog.Error("No valid video format found",
			"PostID", h.postID)
		return nil
	}

	file, err := downloader.FetchBytesFromURL(videoInfo.MasterURL)
	if err != nil {
		slog.Error("Failed to download video",
			"PostID", h.postID,
			"Error", err.Error())
		return nil
	}

	return []models.InputMedia{&models.InputMediaVideo{
		Media: "attach://" + utils.SanitizeString(
			fmt.Sprintf("SmudgeLord-Xiaohongshu_%s", h.postID)),
		Width:             videoInfo.Width,
		Height:            videoInfo.Height,
		Duration:          videoInfo.Duration / 1000,
		SupportsStreaming: true,
		MediaAttachment:   bytes.NewBuffer(file),
	}}
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

func (h *Handler) downloadImages(noteData Note) []models.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
		err   error
	}

	mediaCount := len(noteData.Note.ImageList)
	mediaItems := make([]models.InputMedia, mediaCount)
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
				slog.Error("Failed to download image",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error())

			}
			results <- mediaResult{index, file, err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index,
				"Error", result.err.Error())
			continue
		}
		if result.file != nil {
			if noteData.Note.ImageList[result.index].LivePhoto {
				mediaItems[result.index] = &models.InputMediaVideo{
					Media: "attach://" + utils.SanitizeString(
						fmt.Sprintf("SmudgeLord-Xiaohongshu_%d_%s", result.index, h.postID)),
					SupportsStreaming: true,
					MediaAttachment:   bytes.NewBuffer(result.file),
				}
			} else {
				mediaItems[result.index] = &models.InputMediaPhoto{
					Media: "attach://" + utils.SanitizeString(
						fmt.Sprintf("SmudgeLord-Xiaohongshu_%d_%s", result.index, h.postID)),
					MediaAttachment: bytes.NewBuffer(result.file),
				}
			}
		}
	}

	return mediaItems
}
