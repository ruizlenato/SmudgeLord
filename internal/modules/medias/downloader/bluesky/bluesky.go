package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
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

func Handle(message *telegram.NewMessage) ([]telegram.InputMedia, []string) {
	username, postID := getUsernameAndPostID(message.Text())
	if postID == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, postID}
	}

	blueskyData := getBlueskyData(username, postID)
	if blueskyData == nil {
		return nil, []string{}
	}

	caption := getCaption(blueskyData)

	return processMedia(blueskyData, message), []string{caption, postID}
}

func getUsernameAndPostID(url string) (username, postID string) {
	if matches := regexp.MustCompile(`([^/?#]+)/post/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 3 {
		return matches[1], matches[2]
	}
	return username, postID
}

func getBlueskyData(username, postID string) BlueskyData {
	response, err := utils.Request("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		},
		Query: map[string]string{
			"uri":   fmt.Sprintf("at://%s/app.bsky.feed.post/%s", username, postID),
			"depth": "0",
		},
	})
	defer response.Body.Close()

	if err != nil || response.Body == nil {
		return nil
	}

	var blueskyData BlueskyData
	err = json.NewDecoder(response.Body).Decode(&blueskyData)
	if err != nil {
		log.Print("Bluesky —  Error unmarshalling JSON: ", err)
	}

	return blueskyData
}

func getCaption(bluesky BlueskyData) string {
	return fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n%s",
		bluesky.Thread.Post.Author.DisplayName,
		bluesky.Thread.Post.Author.Handle,
		bluesky.Thread.Post.Record.Text)
}

type InputMedia struct {
	File      *os.File
	Thumbnail *os.File
}

func processMedia(blueskyData BlueskyData, message *telegram.NewMessage) []telegram.InputMedia {
	switch {
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "image"):
		return handleImage(blueskyData.Thread.Post.Embed.Images, message)
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "video"):
		return handleVideo(blueskyData, message)
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "recordWithMedia"):
		if strings.Contains(blueskyData.Thread.Post.Embed.Media.Type, "image") {
			return handleImage(blueskyData.Thread.Post.Embed.Media.Images, message)
		}
		if strings.Contains(blueskyData.Thread.Post.Embed.Media.Type, "video") {
			return handleVideo(blueskyData, message)
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

func handleVideo(blueskyData BlueskyData, message *telegram.NewMessage) []telegram.InputMedia {
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
	defer response.Body.Close()

	if err != nil || response.Body == nil {
		log.Print("Bluesky — Error requesting playlist: ", err)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Print("Bluesky — Error reading response body: ", err)
		return nil
	}

	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(bodyBytes), true)
	if err != nil {
		log.Print("Bluesky — Failed to decode m3u8 playlist: ", err)
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
		log.Printf("Bluesky — Error parsing resolution: %s", err)
		return nil
	}

	file, err := downloader.Downloader(url)
	if err != nil {
		log.Printf("Bluesky — Error downloading video from %s: %s", url, err)
		return nil
	}

	thumbnail, err := downloader.Downloader(thumbnailURL)
	if err != nil {
		log.Printf("Bluesky — Error downloading thumbnail from %s: %s", thumbnailURL, err)
		return nil
	}

	video, err := helpers.UploadDocument(message, helpers.UploadDocumentParams{
		File:  file.Name(),
		Thumb: thumbnail.Name(),
		Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
			SupportsStreaming: true,
			W:                 int32(width),
			H:                 int32(height),
		}},
	})
	if err != nil {
		fmt.Println("Bluesky — Error uploading video: ", err)
	}

	return []telegram.InputMedia{&video}
}

func handleImage(blueskyImages []Image, message *telegram.NewMessage) []telegram.InputMedia {
	type mediaResult struct {
		index int
		file  *os.File
		err   error
	}

	mediaCount := len(blueskyImages)
	mediaItems := make([]telegram.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range blueskyImages {
		go func(index int, media Image) {
			file, err := downloader.Downloader(media.Fullsize)
			if err != nil {
				log.Printf("BlueSky — Error downloading file from %s: %s", media.Fullsize, err)
			}
			results <- mediaResult{index, file, err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			continue
		}
		if result.file != nil {
			photo, _ := helpers.UploadPhoto(message, helpers.UploadPhotoParams{File: result.file.Name()})
			mediaItems[result.index] = &photo
		}
	}

	return mediaItems
}
