package bluesky

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"

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
	body := utils.Request("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		},
		Query: map[string]string{
			"uri":   fmt.Sprintf("at://%s/app.bsky.feed.post/%s", username, postID),
			"depth": "10",
		},
	}).Body()
	if body == nil {
		return nil
	}

	var blueskyData BlueskyData
	err := json.Unmarshal(body, &blueskyData)
	if err != nil {
		log.Print("Bluesky: Error unmarshalling JSON: ", err)
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
	type mediaResult struct {
		index int
		file  *os.File
		err   error
	}

	mediaCount := len(blueskyData.Thread.Post.Embed.Images)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range blueskyData.Thread.Post.Embed.Images {
		go func(index int, media Image) {
			file, err := downloader.Downloader(media.Fullsize)
			if err != nil {
				log.Printf("BlueSky: Error downloading file from %s: %s", media.Fullsize, err)
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
