package tiktok

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(message telego.Message) ([]telego.InputMedia, []string) {
	postID := getPostID(message.Text)
	if postID == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, postID}
	}

	tikTokData, err := getTikTokData(postID)
	if err != nil {
		log.Print(err)
		return nil, []string{}
	}

	caption := getCaption(tikTokData)

	if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
		return downloadImages(tikTokData), []string{caption, postID}
	}
	return downloadVideo(tikTokData), []string{caption, postID}
}

func getPostID(url string) (postID string) {
	resp, err := http.Get(url)
	if err != nil {
		return postID
	}
	defer resp.Body.Close()
	matches := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`).FindStringSubmatch(resp.Request.URL.String())
	if len(matches) > 1 {
		return matches[1]
	}

	return postID
}

func getTikTokData(postID string) (TikTokData, error) {
	body := utils.Request("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestParams{
		Method: "OPTIONS",
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		},
		Query: map[string]string{
			"iid":             "7318518857994389254",
			"device_id":       "7318517321748022790",
			"channel":         "googleplay",
			"version_code":    "300904",
			"device_platform": "android",
			"device_type":     "ASUS_Z01QD",
			"os_version":      "9",
			"aweme_id":        postID,
			"aid":             "1128",
		},
	}).Body()

	if body == nil {
		return nil, errors.New("no response body")
	}

	var tikTokData TikTokData
	err := json.Unmarshal(body, &tikTokData)
	if err != nil {
		return nil, err
	}

	return tikTokData, nil
}

func getCaption(tikTokData TikTokData) string {
	if len(tikTokData.AwemeList) == 0 {
		return ""
	}
	if tikTokData.AwemeList[0].Author.Nickname != nil && tikTokData.AwemeList[0].Desc != nil {
		return fmt.Sprintf("<b>%s</b>:\n%s", *tikTokData.AwemeList[0].Author.Nickname, *tikTokData.AwemeList[0].Desc)
	}
	return ""
}

func downloadImages(tikTokData TikTokData) []telego.InputMedia {
	type mediaResult struct {
		index int
		file  *os.File
		err   error
	}

	mediaCount := len(tikTokData.AwemeList[0].ImagePostInfo.Images)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range tikTokData.AwemeList[0].ImagePostInfo.Images {
		go func(index int, media Image) {
			file, err := downloader.Downloader(media.DisplayImage.URLList[1])
			if err != nil {
				log.Printf("TikTok: Error downloading file from %s: %s", tikTokData.AwemeList[0].AwemeID, err)
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

func downloadVideo(tikTokData TikTokData) []telego.InputMedia {
	file, err := downloader.Downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
	if err != nil {
		log.Printf("TikTok: Error downloading video from %s: %s", tikTokData.AwemeList[0].AwemeID, err)
		return nil
	}

	thumbnail, err := downloader.Downloader(tikTokData.AwemeList[0].Video.Cover.URLList[0])
	if err != nil {
		log.Printf("TikTok: Error downloading thumbnail from %s: %s", tikTokData.AwemeList[0].AwemeID, err)
		return nil
	}

	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: file},
		Thumbnail:         &telego.InputFile{File: thumbnail},
		Width:             tikTokData.AwemeList[0].Video.PlayAddr.Width,
		Height:            tikTokData.AwemeList[0].Video.PlayAddr.Height,
		SupportsStreaming: true,
	}}
}
