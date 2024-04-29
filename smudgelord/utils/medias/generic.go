package medias

import (
	"encoding/json"
	"log"

	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego/telegoutil"
)

type GenericData struct {
	URL     string  `json:"url"`
	Message *string `json:"message"`
}

func (dm *DownloadMedia) Generic(url string) {
	var genericData GenericData
	body := utils.RequestGET("https://scrapper.ruizlenato.workers.dev/"+url, utils.RequestGETParams{}).Body()
	if body == nil {
		return
	}
	err := json.Unmarshal(body, &genericData)
	if err != nil {
		log.Printf("[generic/Generic] Error unmarshalling Generic Data: %v", err)
		return
	}

	if genericData.Message != nil && *genericData.Message == "Invalid link" {
		return
	}

	file, err := Downloader(genericData.URL)
	if err != nil {
		log.Print("[generic/Generic] Error downloading file:", err)
		return
	}

	dm.MediaItems = append(dm.MediaItems, telegoutil.MediaVideo(telegoutil.File(file)))
}
