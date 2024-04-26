package medias

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sync"

	"smudgelord/smudgelord/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

type TikTokData *struct {
	AwemeList []Aweme `json:"aweme_list"`
}

type Aweme struct {
	AwemeID       string        `json:"aweme_id"`
	Desc          *string       `json:"desc"`
	Author        Author        `json:"author,omitempty"`
	Music         Music         `json:"music,omitempty"`
	Video         Video         `json:"video,omitempty"`
	ImagePostInfo ImagePostInfo `json:"image_post_info,omitempty"`
	ShareURL      string        `json:"share_url"`
	AwemeType     int           `json:"aweme_type"`
}

type Author struct {
	Nickname     string       `json:"nickname"`
	UniqueID     string       `json:"unique_id"`
	AvatarLarger AvatarLarger `json:"avatar_larger"`
}

type AvatarLarger struct {
	URLList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

type Music struct {
	Title      string     `json:"title"`
	Author     string     `json:"author"`
	Album      string     `json:"album"`
	CoverLarge CoverLarge `json:"cover_large"`
	PlayURL    PlayURL    `json:"play_url"`
}

type CoverLarge struct {
	URI       string   `json:"uri"`
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	URLPrefix any      `json:"url_prefix"`
}

type PlayURL struct {
	URI       string   `json:"uri"`
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	URLPrefix any      `json:"url_prefix"`
}

type Video struct {
	PlayAddr PlayAddr `json:"play_addr"`
	Cover    Cover    `json:"cover"`
	Height   int      `json:"height"`
	Width    int      `json:"width"`
}

type ImagePostInfo struct {
	Images []Image `json:"images"`
}

type Image struct {
	DisplayImage Cover `json:"display_image"`
	Thumbnail    Cover `json:"thumbnail"`
}

type PlayAddr struct {
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	DataSize  int      `json:"data_size"`
	FileHash  string   `json:"file_hash"`
	URLPrefix any      `json:"url_prefix"`
}

type Cover struct {
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	URLPrefix any      `json:"url_prefix"`
}

func (dm *DownloadMedia) TikTok(url string) {
	var VideoID string

	res, err := http.Get(url)
	if err != nil {
		log.Print("[tiktok/TikTok] Error getting TikTok URL:", err)
		return
	}
	matches := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`).FindStringSubmatch(res.Request.URL.String())
	if len(matches) == 2 {
		VideoID = matches[1]
	} else {
		return
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
		"aweme_id":        string(VideoID),
		"aid":             "1128",
	}

	body := utils.RequestGET("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestGETParams{Query: query, Headers: headers}).Body()
	if body == nil {
		return
	}
	var tikTokData TikTokData
	err = json.Unmarshal(body, &tikTokData)
	if err != nil {
		log.Printf("[tiktok/TikTok] Error unmarshalling TikTok data: %v", err)
		return
	}

	if tikTokData == nil {
		return
	}

	if tikTokData.AwemeList[0].Desc != nil {
		dm.Caption = *tikTokData.AwemeList[0].Desc
	}

	if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
		var wg sync.WaitGroup
		wg.Add(len(tikTokData.AwemeList[0].ImagePostInfo.Images))

		medias := make(map[int]*os.File)
		for i, media := range tikTokData.AwemeList[0].ImagePostInfo.Images {
			go func(index int, media Image) {
				defer wg.Done()
				file, err := downloader(media.DisplayImage.URLList[1])
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
		dm.MediaItems = make([]telego.InputMedia, len(medias))

		// Process medias after all downloads are complete
		for index, file := range medias {
			if file != nil {
				dm.MediaItems[index] = telegoutil.MediaPhoto(telegoutil.File(file))
			}
		}
	} else {
		file, err := downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
		if err != nil {
			log.Print("[tiktok/TikTok] Error downloading video:", err)
			return
		}
		dm.MediaItems = append(dm.MediaItems, telegoutil.MediaVideo(
			telegoutil.File(file)).WithWidth(tikTokData.AwemeList[0].Video.PlayAddr.Width).WithHeight(tikTokData.AwemeList[0].Video.PlayAddr.Height),
		)
	}
}
