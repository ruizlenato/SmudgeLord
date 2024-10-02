package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/grafov/m3u8"
	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) ([]telego.InputMedia, []string) {
	username, postID := getUsernameAndPostID(text)
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

	return processMedia(blueskyData), []string{caption, postID}
}

func getUsernameAndPostID(url string) (username, postID string) {
	if matches := regexp.MustCompile(`([^/?#]+)/post/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 3 {
		return matches[1], matches[2]
	}
	return username, postID
}

func getBlueskyData(username, postID string) BlueskyData {
	request, response, err := utils.Request("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", utils.RequestParams{
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
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response.Body() == nil {
		return nil
	}

	var blueskyData BlueskyData
	err = json.Unmarshal(response.Body(), &blueskyData)
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

func processMedia(blueskyData BlueskyData) []telego.InputMedia {
	switch {
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "image"):
		return handleImage(blueskyData.Thread.Post.Embed.Media.Images)
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "video"):
		return handleVideo(blueskyData)
	case strings.Contains(blueskyData.Thread.Post.Embed.Type, "recordWithMedia"):
		if strings.Contains(blueskyData.Thread.Post.Embed.Media.Type, "image") {
			return handleImage(blueskyData.Thread.Post.Embed.Media.Images)
		}
		if strings.Contains(blueskyData.Thread.Post.Embed.Media.Type, "video") {
			return handleVideo(blueskyData)
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

func handleVideo(blueskyData BlueskyData) []telego.InputMedia {
	playlistURL, thumbnailURL := getPlaylistAndThumbnailURLs(blueskyData)
	if playlistURL == "" || thumbnailURL == "" {
		return nil
	}

	if !strings.HasPrefix(playlistURL, "https://video.bsky.app/") {
		return nil
	}

	request, response, err := utils.Request(playlistURL, utils.RequestParams{
		Method: "GET",
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		log.Print("Bluesky — Error requesting playlist: ", err)
	}

	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(response.Body()), true)
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
		string(request.URI().Scheme()),
		string(request.URI().Host()),
		path.Dir(string(request.URI().Path())),
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

	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: file},
		Thumbnail:         &telego.InputFile{File: thumbnail},
		Width:             width,
		Height:            height,
		SupportsStreaming: true,
	}}
}

func handleImage(blueskyImages []Image) []telego.InputMedia {
	type mediaResult struct {
		index int
		file  *os.File
		err   error
	}

	mediaCount := len(blueskyImages)
	mediaItems := make([]telego.InputMedia, mediaCount)
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
			mediaItems[result.index] = &telego.InputMediaPhoto{
				Type:  telego.MediaTypePhoto,
				Media: telego.InputFile{File: result.file},
			}
		}
	}

	return mediaItems
}
