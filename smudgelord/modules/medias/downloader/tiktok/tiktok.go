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

	"smudgelord/smudgelord/modules/medias/downloader"
	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
)

func TikTok(url string) ([]telego.InputMedia, string) {
	var mediaItems []telego.InputMedia
	var caption string

	res, err := http.Get(url)
	if err != nil {
		log.Printf("[tiktok/TikTok] Error getting TikTok URL: %v", err)
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
		log.Printf("[tiktok/TikTok] No response body for video ID: %s", videoID)
		return nil, caption
	}

	var tikTokData TikTokData
	err = json.Unmarshal(body, &tikTokData)
	if err != nil {
		log.Printf("[tiktok/TikTok] Error unmarshalling TikTok data: %v", err)
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
					log.Print("[tiktok/TikTok] Error downloading photo:", err)
					// Use index as key to store nil for failed downloads
					medias[index] = nil
					return
				}
				// Use index as key to store downloaded file
				medias[index] = file
			}(i, media)
		}

		wg.Wait()
		mediaItems = make([]telego.InputMedia, len(medias))

		// Process medias after all downloads are complete
		for index, file := range medias {
			if file != nil {
				mediaItems[index] = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: file},
				}
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
			log.Print("[tiktok/TikTok] Error downloading thumbnail:", err)
			return nil, caption
		}

		mediaItems = append(mediaItems, &telego.InputMediaVideo{
			Type:      telego.MediaTypeVideo,
			Media:     telego.InputFile{File: file},
			Thumbnail: &telego.InputFile{File: thumbnail},
			Width:     tikTokData.AwemeList[0].Video.PlayAddr.Width,
			Height:    tikTokData.AwemeList[0].Video.PlayAddr.Height,
		})
	}

	return mediaItems, caption
}
