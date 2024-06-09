package generic

import (
	"encoding/json"
	"log"

	"smudgelord/smudgelord/modules/medias/downloader"
	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

func Generic(url string) ([]telego.InputMedia, string) {
	var genericData GenericData
	var mediaItems []telego.InputMedia
	var caption string

	body := utils.Request("https://scrapper.ruizlenato.workers.dev/", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"url": url,
		},
	}).Body()
	if body == nil {
		return nil, caption
	}

	err := json.Unmarshal(body, &genericData)
	if err != nil {
		log.Printf("[generic/Generic] Error unmarshalling Generic Data: %v", err)
		return nil, caption
	}

	if genericData.Message != nil && *genericData.Message == "Invalid link" {
		return nil, caption
	}

	file, err := downloader.Downloader(genericData.URL)
	if err != nil {
		log.Print("[generic/Generic] Error downloading file:", err)
		return nil, caption
	}

	mediaItems = append(mediaItems, telegoutil.MediaVideo(telegoutil.File(file)))
	return mediaItems, caption
}
