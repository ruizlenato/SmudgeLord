package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/grafov/m3u8"
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
	if !handler.setUsernameAndPostID(message.Text()) {
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

	return handler.processMedia(blueskyData, message), []string{caption, handler.postID}
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
			"Content-Type": "application/json",
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
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

	var blueskyData BlueskyData
	err = json.NewDecoder(response.Body).Decode(&blueskyData)
	if err != nil {
		slog.Error("Failed to decode Bluesky data",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}

	return blueskyData
}

func getCaption(bluesky BlueskyData) string {
	return fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n%s",
		bluesky.Thread.Post.Author.DisplayName,
		bluesky.Thread.Post.Author.Handle,
		bluesky.Thread.Post.Record.Text)
}

func (h *Handler) processMedia(blueskyData BlueskyData, message *telegram.NewMessage) []telegram.InputMedia {
	switch {
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "image"):
		return h.handleImages(blueskyData.Thread.Post.Embed.Images, message)
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "video"):
		return h.handleVideo(blueskyData, message)
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "recordWithMedia"):
		if strings.Contains(blueskyData.Thread.Post.Embed.Media.Type, "image") {
			return h.handleImages(blueskyData.Thread.Post.Embed.Media.Images, message)
		}
		if strings.Contains(blueskyData.Thread.Post.Embed.Media.Type, "video") {
			return h.handleVideo(blueskyData, message)
		}
		return nil
	}

	return nil
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

func getPlaylistAndThumbnailURLs(blueskyData BlueskyData) (string, string) {
	if blueskyData.Thread.Post.Embed.Playlist != nil {
		return *blueskyData.Thread.Post.Embed.Playlist, *blueskyData.Thread.Post.Embed.Thumbnail
	}
	return blueskyData.Thread.Post.Embed.Media.Playlist, blueskyData.Thread.Post.Embed.Media.Thumbnail
}

func (h *Handler) handleVideo(blueskyData BlueskyData, message *telegram.NewMessage) []telegram.InputMedia {
	playlistURL, thumbnailURL := getPlaylistAndThumbnailURLs(blueskyData)
	if playlistURL == "" || thumbnailURL == "" {
		return nil
	}

	if !strings.HasPrefix(playlistURL, "https://video.bsky.app/") {
		return nil
	}

	response, err := utils.Request(playlistURL, utils.RequestParams{
		Method: "GET",
	})

	if err != nil || response.Body == nil {
		slog.Error("Failed to request playlist",
			"Post Info", []string{h.username, h.postID},
			"Playlist URL", playlistURL,
			"Error", err.Error())
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Error("Failed to read playlist response body",
			"Post Info", []string{h.username, h.postID},
			"Playlist URL", playlistURL,
			"Error", err.Error())
		return nil
	}

	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(bodyBytes), true)
	if err != nil {
		slog.Error("Failed to decode m3u8 playlist",
			"Post Info", []string{h.username, h.postID},
			"Playlist URL", playlistURL,
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
		response.Request.URL.Scheme,
		response.Request.URL.Host,
		path.Dir(response.Request.URL.Path),
		highestBandwidthVariant.URI)

	width, height, err := parseResolution(highestBandwidthVariant.Resolution)
	if err != nil {
		slog.Error("Failed to parse video resolution",
			"Post Info", []string{h.username, h.postID},
			"Resolution", highestBandwidthVariant.Resolution,
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

	video, err := helpers.UploadVideo(message, helpers.UploadVideoParams{
		File:              file,
		Thumb:             thumbnail,
		SupportsStreaming: true,
		Width:             int32(width),
		Height:            int32(height),
	})
	if err != nil {
		slog.Error("Failed to upload video",
			"Post Info", []string{h.username, h.postID},
			"Video URL", url,
			"Error", err.Error())
		return nil
	}

	return []telegram.InputMedia{&video}
}

func (h *Handler) handleImages(blueskyImages []Image, message *telegram.NewMessage) []telegram.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
	}

	mediaCount := len(blueskyImages)
	mediaItems := make([]telegram.InputMedia, mediaCount)
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
			results <- mediaResult{index, file}
		}(i, media)
	}

	for range mediaCount {
		result := <-results
		if result.file != nil {
			photo, _ := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
				File: result.file,
			})
			mediaItems[result.index] = &photo
		}
	}

	return mediaItems
}
