package tiktok

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(message *telegram.NewMessage) ([]telegram.InputMedia, []string) {
	postID := getPostID(message.Text())
	if postID == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, postID}
	}

	tikTokData := getTikTokData(postID)
	if tikTokData == nil {
		return nil, []string{}
	}

	caption := getCaption(tikTokData)

	if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
		return downloadImages(tikTokData, message), []string{caption, postID}
	}
	return downloadVideo(tikTokData, message), []string{caption, postID}
}

func getPostID(url string) (postID string) {
	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	response, err := retryCaller.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})
	defer response.Body.Close()

	if err != nil {
		return postID
	}

	matches := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`).FindStringSubmatch(response.Request.URL.String())
	if len(matches) > 1 {
		return matches[1]
	}

	return postID
}

func getTikTokData(postID string) TikTokData {
	response, err := utils.Request("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestParams{
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
	})
	defer response.Body.Close()

	if err != nil || response.Body == nil {
		return nil
	}

	var tikTokData TikTokData
	err = json.NewDecoder(response.Body).Decode(&tikTokData)
	if err != nil {
		return nil
	}

	return tikTokData
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

func downloadImages(tikTokData TikTokData, message *telegram.NewMessage) []telegram.InputMedia {
	type mediaResult struct {
		index int
		file  *os.File
		err   error
	}

	mediaCount := len(tikTokData.AwemeList[0].ImagePostInfo.Images)
	mediaChan := make(chan mediaResult, mediaCount)
	medias := make([]*os.File, mediaCount)
	mediaItems := make([]telegram.InputMedia, mediaCount)

	for i, media := range tikTokData.AwemeList[0].ImagePostInfo.Images {
		go func(index int, media Image) {
			file, err := downloader.Downloader(media.DisplayImage.URLList[1])
			if err != nil {
				log.Print("[tiktok/TikTok] Error downloading photo: ", err)
			}
			mediaChan <- mediaResult{index, file, err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-mediaChan
		medias[result.index] = result.file
	}

	uploadChan := make(chan struct {
		index int
		media telegram.InputMedia
		err   error
	}, mediaCount)

	var (
		seqnoMutex sync.Mutex
		seqno      int
	)

	for i, file := range medias {
		go func(index int, file *os.File) {
			seqnoMutex.Lock()
			defer seqnoMutex.Unlock()
			seqno++

			if file != nil {
				photo, err := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
					File: file.Name(),
				})
				if err != nil {
					log.Print("[tiktok/TikTok] Error uploading photo: ", err)
				}
				uploadChan <- struct {
					index int
					media telegram.InputMedia
					err   error
				}{index, &photo, err}
			} else {
				uploadChan <- struct {
					index int
					media telegram.InputMedia
					err   error
				}{index, nil, nil}
			}
		}(i, file)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-uploadChan
		if result.err == nil {
			mediaItems[result.index] = result.media
		}
	}

	return mediaItems
}

func downloadVideo(tikTokData TikTokData, message *telegram.NewMessage) []telegram.InputMedia {
	file, err := downloader.Downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
	if err != nil {
		log.Print("[tiktok/TikTok] Error downloading video:", err)
		return nil
	}

	thumbnail, err := downloader.Downloader(tikTokData.AwemeList[0].Video.Cover.URLList[0])
	if err != nil {
		log.Print("[tiktok/TikTok] Error downloading thumbnail: ", err)
		return nil
	}

	video, err := helpers.UploadDocument(message, helpers.UploadDocumentParams{
		File:  file.Name(),
		Thumb: thumbnail.Name(),
		Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
			SupportsStreaming: true,
			W:                 int32(tikTokData.AwemeList[0].Video.PlayAddr.Width),
			H:                 int32(tikTokData.AwemeList[0].Video.PlayAddr.Height),
		}},
	})
	if err != nil {
		log.Print("[instagram/Instagram] Error uploading video: ", err)
		return nil
	}

	return []telegram.InputMedia{&video}
}
