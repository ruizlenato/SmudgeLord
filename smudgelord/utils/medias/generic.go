package medias

import (
	"encoding/json"
	"log"

	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego/telegoutil"
)

type GenericData struct {
	URL string `json:"url"`
}

func (dm *DownloadMedia) Generic(url string) {
	var genericData GenericData
	body := utils.RequestGET("https://scrapper.ruizlenato.workers.dev/"+url, utils.RequestGETParams{}).Body()
	err := json.Unmarshal(body, &genericData)
	if err != nil {
		log.Printf("Error unmarshalling Generic Data: %v", err)
		return
	}

	file, err := downloader(genericData.URL)
	if err != nil {
		log.Println(err)
		return
	}

	dm.MediaItems = append(dm.MediaItems, telegoutil.MediaVideo(telegoutil.File(file)))
}
