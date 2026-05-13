package substack

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	cacheID := fmt.Sprintf("substack:%s", handler.postID)
	if postInfo, err := downloader.GetMediaCache(cacheID); err == nil {
		return postInfo
	}

	data := handler.getSubstackData()
	if data == nil {
		return downloader.PostInfo{}
	}

	medias, cleanup := handler.processMedia(data)

	return downloader.PostInfo{
		ID:      cacheID,
		Medias:  medias,
		Caption: getCaption(data),
		Cleanup: cleanup,
	}
}

func (h *Handler) setPostID(url string) bool {
	matches := regexp.MustCompile(`(?:^|/)c-(\d+)(?:[/?#]|$)`).FindStringSubmatch(url)
	if len(matches) != 2 {
		return false
	}

	h.postID = matches[1]
	return true
}

func (h *Handler) getSubstackData() APIData {
	response, err := utils.Request(fmt.Sprintf("https://substack.com/api/v1/reader/comment/%s", h.postID), utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent": downloader.GenericHeaders["User-Agent"],
			"Accept":     "application/json",
		},
	})
	if err != nil || response == nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil
	}

	var data APIData
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		slog.Error("Failed to unmarshal Substack JSON",
			"Post Info", h.postID,
			"Error", err.Error())
		return nil
	}

	return data
}

func getCaption(data APIData) string {
	body := html.EscapeString(data.Item.Comment.Body)
	name := html.EscapeString(data.Item.Comment.Name)
	handle := strings.TrimPrefix(data.Item.Comment.Handle, "@")
	escapedHandle := html.EscapeString(handle)

	switch {
	case name != "" && escapedHandle != "":
		return fmt.Sprintf("<b><a href='https://substack.com/@%s'>%s</a></b>:\n%s", escapedHandle, name, body)
	case name != "":
		return fmt.Sprintf("<b>%s</b>:\n%s", name, body)
	case escapedHandle != "":
		return fmt.Sprintf("<b>@%s</b>:\n%s", escapedHandle, body)
	default:
		return body
	}
}

func (h *Handler) processMedia(data APIData) ([]gotgbot.InputMedia, func()) {
	medias := make([]gotgbot.InputMedia, 0, len(data.Item.Comment.Attachments))
	cleanups := make([]func(), 0, len(data.Item.Comment.Attachments))
	for i, attachment := range data.Item.Comment.Attachments {
		if attachment.Type != "image" || attachment.ImageURL == "" {
			continue
		}

		stream, cleanup, err := h.downloadAttachmentImage(attachment.ImageURL)
		if err != nil {
			slog.Error("Failed to download Substack image",
				"Post Info", h.postID,
				"Index", i,
				"Image URL", attachment.ImageURL,
				"Error", err.Error())
			continue
		}

		filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Substack_%d_%s", i, h.postID))
		medias = append(medias, &gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromReader(filename, stream),
		})
		cleanups = append(cleanups, cleanup)
	}

	return medias, downloader.CombineCleanups(cleanups...)
}

func (h *Handler) downloadAttachmentImage(imageURL string) (io.ReadCloser, func(), error) {
	var lastErr error
	for _, candidate := range buildImageCandidates(imageURL) {
		stream, cleanup, err := downloader.FetchStreamFromURL(candidate)
		if err == nil {
			return stream, cleanup, nil
		}
		if cleanup != nil {
			cleanup()
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no valid image candidates")
	}
	return nil, nil, lastErr
}

func buildImageCandidates(imageURL string) []string {
	trimmed := strings.TrimSpace(imageURL)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return candidates
	}

	ext := strings.ToLower(path.Ext(parsed.Path))
	if strings.HasSuffix(parsed.Host, "substack-post-media.s3.amazonaws.com") || ext == ".heic" || ext == ".heif" {
		cdnURL := "https://substackcdn.com/image/fetch/f_auto,q_auto:good/" + url.PathEscape(trimmed)
		candidates = []string{cdnURL, trimmed}
	}

	return candidates
}
