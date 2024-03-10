package medias

import (
	"fmt"
	"regexp"

	"github.com/mymmrac/telego"
)

type DownloadMedia struct {
	MediaItems []telego.InputMedia
	Caption    string
}

func NewDownloadMedia() *DownloadMedia {
	return &DownloadMedia{
		MediaItems: make([]telego.InputMedia, 0, 10),
	}
}

func (dm *DownloadMedia) Download(url string) ([]telego.InputMedia, string) {
	if match, _ := regexp.MatchString("(twitter|x).com/", url); match {
		dm.Twitter(url)
	} else if match, _ := regexp.MatchString("instagram.com/", url); match {
		dm.Instagram(url)
	} else if match, _ := regexp.MatchString("tiktok.com/", url); match {
		dm.TikTok(url)
	}

	if dm.MediaItems != nil && dm.Caption == "" {
		dm.Caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url)
	}

	return dm.MediaItems, dm.Caption
}
