package xiaohongshu

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	postURL := handler.setPostID(text)
	if postURL == "" {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(handler.postID); err == nil {
		return postInfo
	}

	xiaohongshuData := handler.getXiaohongshuData(postURL)
	if xiaohongshuData == nil {
		return downloader.PostInfo{}
	}

	noteData, noteDataExists := xiaohongshuData.Note.NoteDetailMap[handler.postID]
	if !noteDataExists {
		return downloader.PostInfo{}
	}

	handler.username = noteData.Note.User.Nickname

	switch noteData.Note.Type {
	case "video":
		medias, cleanup := handler.handleVideo(noteData)
		return downloader.PostInfo{
			ID:      handler.postID,
			Medias:  medias,
			Caption: getCaption(noteData),
			Cleanup: cleanup,
		}
	case "normal":
		medias, cleanup := handler.handleImages(noteData)
		return downloader.PostInfo{
			ID:      handler.postID,
			Medias:  medias,
			Caption: getCaption(noteData),
			Cleanup: cleanup,
		}
	default:
		return downloader.PostInfo{}
	}
}

func (h *Handler) setPostID(text string) string {
	postURL := h.getPostURL(text)
	if postURL == "" {
		return ""
	}

	postIDRegex := regexp.MustCompile(`/(?:explore|item)/(\w+)`)
	if matches := postIDRegex.FindStringSubmatch(postURL); len(matches) > 1 {
		h.postID = matches[1]
		return postURL
	}
	return ""
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

func (h *Handler) getXiaohongshuData(url string) XiaohongshuData {
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
	caption := fmt.Sprintf("<b>%s</b>:", html.EscapeString(noteData.Note.User.Nickname))
	if noteData.Note.Title != "" {
		caption += fmt.Sprintf("\n<b>%s</b>", html.EscapeString(noteData.Note.Title))
	}
	if noteData.Note.Desc != "" {
		caption += fmt.Sprintf("\n%s", html.EscapeString(noteData.Note.Desc))
	}

	return caption
}

func (h *Handler) handleVideo(noteData Note) ([]gotgbot.InputMedia, func()) {
	videoInfo := h.findFirstAvailableVideoFormat(noteData.Note.Video.Media.Stream)
	if videoInfo == nil {
		slog.Error("No valid video format found",
			"Post Info", []string{h.username, h.postID})
		return nil, nil
	}

	stream, cleanup, err := downloader.FetchStreamFromURL(videoInfo.MasterURL)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Xiaohongshu_%s_%s", h.username, h.postID))
	return []gotgbot.InputMedia{&gotgbot.InputMediaVideo{
		Media:             downloader.InputFileFromReader(filename, stream),
		Width:             int64(videoInfo.Width),
		Height:            int64(videoInfo.Height),
		Duration:          int64(videoInfo.Duration / 1000),
		SupportsStreaming: true,
	}}, cleanup
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

func (h *Handler) handleImages(noteData Note) ([]gotgbot.InputMedia, func()) {
	type mediaResult struct {
		index   int
		media   gotgbot.InputMedia
		cleanup func()
		err     error
	}

	mediaCount := len(noteData.Note.ImageList)
	mediaItems := make([]gotgbot.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range noteData.Note.ImageList {
		go func(index int, media Images) {
			url := media.URLDefault
			if media.LivePhoto {
				videoInfo := h.findFirstAvailableVideoFormat(media.Stream)
				url = videoInfo.MasterURL
			}

			stream, cleanup, err := downloader.FetchStreamFromURL(url)
			if err != nil {
				slog.Error("Failed to download image",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error())
				results <- mediaResult{index: index, err: err}
				return
			}

			sanitized := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Xiaohongshu_%d_%s_%s", index, h.username, h.postID))
			if media.LivePhoto {
				results <- mediaResult{
					index: index,
					media: &gotgbot.InputMediaVideo{
						Media:             downloader.InputFileFromReader(sanitized, stream),
						SupportsStreaming: true,
					},
					cleanup: cleanup,
				}
				return
			}

			results <- mediaResult{
				index: index,
				media: &gotgbot.InputMediaPhoto{
					Media: downloader.InputFileFromReader(sanitized, stream),
				},
				cleanup: cleanup,
			}
		}(i, media)
	}

	var cleanups []func()
	for range mediaCount {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index,
				"Error", result.err.Error())
			continue
		}
		if result.cleanup != nil {
			cleanups = append(cleanups, result.cleanup)
		}
		if result.media != nil {
			mediaItems[result.index] = result.media
		}
	}

	return mediaItems, downloader.CombineCleanups(cleanups...)
}
