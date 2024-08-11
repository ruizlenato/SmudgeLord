package tiktok

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sync"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/utils"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"

	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
)

func TikTok(url string, message *telegram.NewMessage) ([]telegram.InputMedia, string) {
	var mediaItems []telegram.InputMedia
	var caption string

	res, err := http.Get(url)
	if err != nil {
		log.Print("[tiktok/TikTok] Error getting TikTok URL: ", err)
		return nil, caption
	}
	defer res.Body.Close()

	matches := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`).FindStringSubmatch(res.Request.URL.String())
	if len(matches) != 2 {
		return nil, caption
	}
	videoID := matches[1]

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
			"aweme_id":        videoID,
			"aid":             "1128",
		},
	}).Body()

	if body == nil {
		log.Print("[tiktok/TikTok] No response body for video ID: ", videoID)
		return nil, caption
	}

	var tikTokData TikTokData
	err = json.Unmarshal(body, &tikTokData)
	if err != nil {
		log.Print("[tiktok/TikTok] Error unmarshalling TikTok data: ", err)
		return nil, caption
	}

	if len(tikTokData.AwemeList) == 0 {
		return nil, caption
	}

	if tikTokData.AwemeList[0].Author.Nickname != nil && tikTokData.AwemeList[0].Desc != nil {
		caption = fmt.Sprintf("<b>%s</b>:\n%s", *tikTokData.AwemeList[0].Author.Nickname, *tikTokData.AwemeList[0].Desc)
	}

	if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
		var wg sync.WaitGroup
		wg.Add(len(tikTokData.AwemeList[0].ImagePostInfo.Images))

		medias := make(map[int]*os.File)
		for i, media := range tikTokData.AwemeList[0].ImagePostInfo.Images {
			go func(index int, media Image) {
				defer wg.Done()
				file, err := downloader.Downloader(media.DisplayImage.URLList[1])
				if err != nil {
					log.Print("[tiktok/TikTok] Error downloading photo: ", err)
					// Use index as key to store nil for failed downloads
					medias[index] = nil
					return
				}
				// Use index as key to store downloaded file
				medias[index] = file
			}(i, media)
		}

		wg.Wait()
		mediaItems = make([]telegram.InputMedia, 0, len(medias))

		// Process medias after all downloads are complete
		for index, file := range medias {
			if file != nil {
				photo, err := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
					File: file.Name(),
				})
				if err != nil {
					log.Print("[instagram/Instagram] Error uploading video: ", err)
					return nil, caption
				}

				mediaItems[index] = &photo
			}
		}
	} else {
		file, err := downloader.Downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
		if err != nil {
			log.Print("[tiktok/TikTok] Error downloading video:", err)
			return nil, caption
		}

		thumbnail, err := downloader.Downloader(tikTokData.AwemeList[0].Video.Cover.URLList[0])
		if err != nil {
			log.Print("[tiktok/TikTok] Error downloading thumbnail: ", err)
			return nil, caption
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
			return nil, caption
		}

		mediaItems = append(mediaItems, &video)
	}

	return mediaItems, caption
}
