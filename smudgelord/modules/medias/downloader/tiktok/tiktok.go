package tiktok

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sync"

	"smudgelord/smudgelord/modules/medias/downloader"
	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

func TikTok(url string) ([]telego.InputMedia, string) {
	var mediaItems []telego.InputMedia
	var caption string
	var videoID string

	res, err := http.Get(url)
	if err != nil {
		log.Print("[tiktok/TikTok] Error getting TikTok URL:", err)
		return nil, caption
	}
	matches := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`).FindStringSubmatch(res.Request.URL.String())
	if len(matches) == 2 {
		videoID = matches[1]
	} else {
		return nil, caption
	}

	headers := map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0"}
	query := map[string]string{
		"iid":             "7318518857994389254",
		"device_id":       "7318517321748022790",
		"channel":         "googleplay",
		"version_code":    "300904",
		"device_platform": "android",
		"device_type":     "ASUS_Z01QD",
		"os_version":      "9",
		"aweme_id":        string(videoID),
		"aid":             "1128",
	}

	body := utils.RequestGET("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestGETParams{Query: query, Headers: headers}).Body()
	if body == nil {
		return nil, caption
	}
	var tikTokData TikTokData
	err = json.Unmarshal(body, &tikTokData)
	if err != nil {
		log.Printf("[tiktok/TikTok] Error unmarshalling TikTok data: %v", err)
		return nil, caption
	}

	if tikTokData == nil {
		return nil, caption
	}

	if tikTokData.AwemeList[0].Desc != nil {
		caption = *tikTokData.AwemeList[0].Desc
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
				mediaItems[index] = telegoutil.MediaPhoto(telegoutil.File(file))
			}
		}
	} else {
		file, err := downloader.Downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
		if err != nil {
			log.Print("[tiktok/TikTok] Error downloading video:", err)
			return nil, caption
		}
		mediaItems = append(mediaItems, telegoutil.MediaVideo(
			telegoutil.File(file)).WithWidth(tikTokData.AwemeList[0].Video.PlayAddr.Width).WithHeight(tikTokData.AwemeList[0].Video.PlayAddr.Height),
		)
	}

	return mediaItems, caption
}
