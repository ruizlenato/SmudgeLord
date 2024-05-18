package tiktok

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"smudgelord/smudgelord/modules/medias/downloader"
	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

func TikTok(url string) ([]telego.InputMedia, string) {
	var mediaItems []telego.InputMedia
	var caption string

	body := utils.RequestGET("https://scrapper.ruizlenato.workers.dev/", utils.RequestGETParams{Query: map[string]string{"url": url}}).Body()
	if body == nil {
		return nil, caption
	}

	var tikTokData TikTokData
	err := json.Unmarshal(body, &tikTokData)
	if err != nil {
		log.Printf("[tiktok/TikTok] Error unmarshalling TikTok data: %v", err)
		return nil, caption
	}

	if tikTokData == nil {
		return nil, caption
	}

	if tikTokData.Images != nil {
		var wg sync.WaitGroup
		wg.Add(len(*tikTokData.Images))

		medias := make(map[int]*os.File)
		for i, media := range *tikTokData.Images {
			go func(index int, media string) {
				defer wg.Done()
				file, err := downloader.Downloader(media)
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
		file, err := downloader.Downloader(tikTokData.VideoURL)
		if err != nil {
			log.Print("[tiktok/TikTok] Error downloading video:", err)
			return nil, caption
		}
		mediaItems = append(mediaItems, telegoutil.MediaVideo(
			telegoutil.File(file)),
		)
	}
	caption = tikTokData.Title

	return mediaItems, caption
}
