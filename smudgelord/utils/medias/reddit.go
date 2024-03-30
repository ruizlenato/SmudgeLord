package medias

import (
	"encoding/json"
	"log"

	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego/telegoutil"
)

type RedditData struct {
	URL string `json:"url"`
}

func (dm *DownloadMedia) Reddit(url string) {
	var redditData RedditData
	body := utils.RequestGET("https://scrapper.ruizlenato.workers.dev/"+url, utils.RequestGETParams{}).Body()
	err := json.Unmarshal(body, &redditData)
	if err != nil {
		log.Printf("Error unmarshalling Reddit data: %v", err)
		return
	}

	file, err := downloader(redditData.URL)
	if err != nil {
		log.Println(err)
		return
	}

	dm.MediaItems = append(dm.MediaItems, telegoutil.MediaVideo(telegoutil.File(file)))
}
