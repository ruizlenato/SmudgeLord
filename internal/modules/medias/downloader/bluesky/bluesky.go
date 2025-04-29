package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
	"github.com/grafov/m3u8"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	username string
	postID   string
}

func Handle(text string) ([]models.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setUsernameAndPostID(text) {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, handler.postID}
	}

	blueskyData := handler.getBlueskyData()
	if blueskyData == nil {
		return nil, []string{}
	}

	caption := getCaption(blueskyData)

	return handler.processMedia(blueskyData), []string{caption, handler.postID}
}

func (h *Handler) setUsernameAndPostID(url string) bool {
	if matches := regexp.MustCompile(`([^/?#]+)/post/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 3 {
		h.username = matches[1]
		h.postID = matches[2]
		return true
	}

	return false
}

func (h *Handler) getBlueskyData() BlueskyData {
	response, err := utils.Request("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent":   downloader.GenericHeaders["User-Agent"],
			"Content-Type": "application/json",
		},
		Query: map[string]string{
			"uri":   fmt.Sprintf("at://%s/app.bsky.feed.post/%s", h.username, h.postID),
			"depth": "0",
		},
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	var data BlueskyData
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		slog.Error("Failed to unmarshal JSON",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	return data
}

func getCaption(bluesky BlueskyData) string {
	return fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n%s",
		bluesky.Thread.Post.Author.DisplayName,
		bluesky.Thread.Post.Author.Handle,
		bluesky.Thread.Post.Record.Text)
}

func (h *Handler) processMedia(data BlueskyData) []models.InputMedia {
	switch {
	case strings.Contains(data.Thread.Post.Embed.Type, "image"):
		return h.handleImage(data.Thread.Post.Embed.Images)
	case strings.Contains(data.Thread.Post.Embed.Type, "video"):
		return h.handleVideo(data)
	case strings.Contains(data.Thread.Post.Embed.Type, "recordWithMedia"):
		if strings.Contains(data.Thread.Post.Embed.Media.Type, "image") {
			return h.handleImage(data.Thread.Post.Embed.Media.Images)
		}
		if strings.Contains(data.Thread.Post.Embed.Media.Type, "video") {
			return h.handleVideo(data)
		}
		return nil
	default:
		return nil
	}
}

func parseResolution(resolution string) (int, int, error) {
	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid resolution format: %s", resolution)
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %s", parts[0])
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %s", parts[1])
	}

	return width, height, nil
}

func getPlaylistAndThumbnailURLs(data BlueskyData) (string, string) {
	if data.Thread.Post.Embed.Playlist != nil {
		return *data.Thread.Post.Embed.Playlist, *data.Thread.Post.Embed.Thumbnail
	}
	return data.Thread.Post.Embed.Media.Playlist, data.Thread.Post.Embed.Media.Thumbnail
}

func (h *Handler) handleVideo(data BlueskyData) []models.InputMedia {
	playlistURL, thumbnailURL := getPlaylistAndThumbnailURLs(data)
	if playlistURL == "" || thumbnailURL == "" {
		return nil
	}

	if !strings.HasPrefix(playlistURL, "https://video.bsky.app/") {
		return nil
	}

	response, err := utils.Request(playlistURL, utils.RequestParams{
		Method: "GET",
	})
	if err != nil {
		slog.Error("Failed to request playlist",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}
	defer response.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(response.Body, true)
	if err != nil {
		slog.Error("Failed to decode m3u8 playlist",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}

	if listType != m3u8.MASTER {
		return nil
	}

	var highestBandwidthVariant *m3u8.Variant
	for _, variant := range playlist.(*m3u8.MasterPlaylist).Variants {
		if highestBandwidthVariant == nil || variant.Bandwidth > highestBandwidthVariant.Bandwidth {
			highestBandwidthVariant = variant
		}
	}

	url := fmt.Sprintf("%s://%s%s/%s",
		string(response.Request.URL.Scheme),
		string(response.Request.URL.Host),
		path.Dir(string(response.Request.URL.Path)),
		highestBandwidthVariant.URI)

	width, height, err := parseResolution(highestBandwidthVariant.Resolution)
	if err != nil {
		slog.Error("Failed to parse resolution",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	file, err := downloader.FetchBytesFromURL(url)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Video URL", url,
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(thumbnailURL)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error())
		return nil
	}

	thumbnail, err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error())
	}

	return []models.InputMedia{&models.InputMediaVideo{
		Media: "attach://" + utils.SanitizeString(
			fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)),
		Thumbnail: &models.InputFileUpload{
			Filename: utils.SanitizeString(
				fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)),
			Data: bytes.NewBuffer(thumbnail),
		},
		Width:             width,
		Height:            height,
		SupportsStreaming: true,
		MediaAttachment:   bytes.NewBuffer(file),
	}}
}

func (h *Handler) handleImage(blueskyImages []Image) []models.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
		err   error
	}

	mediaCount := len(blueskyImages)
	mediaItems := make([]models.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range blueskyImages {
		go func(index int, media Image) {
			file, err := downloader.FetchBytesFromURL(media.Fullsize)
			if err != nil {
				slog.Error("Failed to download image",
					"Post Info", []string{h.username, h.postID},
					"Image URL", media.Fullsize,
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
			mediaItems[result.index] = &models.InputMediaPhoto{
				Media: "attach://" + utils.SanitizeString(
					fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)),
				MediaAttachment: bytes.NewBuffer(result.file),
			}
		}
	}

	return mediaItems
}
