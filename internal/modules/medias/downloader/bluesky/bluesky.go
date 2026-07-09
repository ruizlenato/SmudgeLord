package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/grafov/m3u8"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	postInfo, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return postInfo
	}

	blueskyData := handler.getBlueskyData()
	if blueskyData == nil {
		return downloader.PostInfo{}
	}

	medias, cleanup := handler.processMedia(blueskyData)

	postInfo = downloader.PostInfo{
		ID:      handler.postID,
		Medias:  medias,
		Caption: getCaption(blueskyData),
	}
	postInfo.Cleanup = downloader.CombineCleanups(postInfo.Cleanup, cleanup)
	return postInfo
}

func (h *Handler) setPostID(url string) bool {
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
		html.EscapeString(bluesky.Thread.Post.Author.DisplayName),
		html.EscapeString(bluesky.Thread.Post.Author.Handle),
		html.EscapeString(bluesky.Thread.Post.Record.Text))
}

func (h *Handler) processMedia(data BlueskyData) ([]gotgbot.InputMedia, func()) {
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
		return nil, nil
	default:
		return nil, nil
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

func (h *Handler) handleVideo(data BlueskyData) ([]gotgbot.InputMedia, func()) {
	playlistURL, thumbnailURL := getPlaylistAndThumbnailURLs(data)
	if playlistURL == "" || thumbnailURL == "" {
		return nil, nil
	}

	if !strings.HasPrefix(playlistURL, "https://video.bsky.app/") {
		return nil, nil
	}

	response, err := utils.Request(playlistURL, utils.RequestParams{
		Method: "GET",
	})
	if err != nil {
		slog.Error("Failed to request playlist",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}
	defer response.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(response.Body, true)
	if err != nil {
		slog.Error("Failed to decode m3u8 playlist",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	if listType != m3u8.MASTER {
		return nil, nil
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
		return nil, nil
	}

	file, cleanup, err := downloader.FetchStreamFromURL(url)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Video URL", url,
			"Error", err.Error())
		return nil, nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(thumbnailURL)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error())
		return nil, cleanup
	}

	thumbnail, err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error())
		return nil, cleanup
	}

	return []gotgbot.InputMedia{&gotgbot.InputMediaVideo{
		Media:                 downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)), file),
		Thumbnail:             downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)), bytes.NewReader(thumbnail)),
		Width:                 int64(width),
		Height:                int64(height),
		SupportsStreaming:     true,
		ShowCaptionAboveMedia: false,
	}}, cleanup
}

func (h *Handler) handleImage(blueskyImages []Image) ([]gotgbot.InputMedia, func()) {
	type mediaResult struct {
		index   int
		file    io.ReadCloser
		cleanup func()
		err     error
	}

	mediaCount := len(blueskyImages)
	mediaItems := make([]gotgbot.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range blueskyImages {
		go func(index int, media Image) {
			file, cleanup, err := downloader.FetchStreamFromURL(media.Fullsize)
			if err != nil {
				slog.Error("Failed to download image",
					"Post Info", []string{h.username, h.postID},
					"Image URL", media.Fullsize,
					"Error", err.Error())
			}
			results <- mediaResult{index, file, cleanup, err}
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
			if result.cleanup != nil {
				cleanups = append(cleanups, result.cleanup)
			}
			continue
		}
		if result.file != nil {
			cleanups = append(cleanups, result.cleanup)
			mediaItems[result.index] = &gotgbot.InputMediaPhoto{
				Media: downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)), result.file),
			}
		}
	}

	nonNil := make([]gotgbot.InputMedia, 0, len(mediaItems))
	for _, m := range mediaItems {
		if m != nil {
			nonNil = append(nonNil, m)
		}
	}

	if len(cleanups) > 0 {
		return nonNil, downloader.CombineCleanups(cleanups...)
	}

	return nonNil, nil
}
