package generic

import (
	"encoding/json"
	"log"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
)

func Generic(url string, message *telegram.NewMessage) ([]telegram.InputMedia, string) {
	var genericData GenericData
	var mediaItems []telegram.InputMedia
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

	video, err := helpers.UploadDocument(message, helpers.UploadDocumentParams{
		File: file.Name(),
	})
	if err != nil {
		log.Print("[instagram/Instagram] Error uploading video:", err)
		return nil, caption
	}

	mediaItems = append(mediaItems, &video)

	return mediaItems, caption
}
