package medias

import (
	"fmt"
	"os"
	"regexp"
	"unicode/utf8"

	"smudgelord/smudgelord/utils"

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
	} else if match, _ := regexp.MatchString("reddit.com/", url); match {
		dm.Reddit(url)
	}

	if dm.MediaItems != nil && dm.Caption == "" {
		dm.Caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url)
	}

	if utf8.RuneCountInString(dm.Caption) > 1024 {
		dm.Caption = truncateUTF8Caption(dm.Caption, url)
	}

	return dm.MediaItems, dm.Caption
}

func downloader(media string) (*os.File, error) {
	body := utils.RequestGET(media, utils.RequestGETParams{}).Body()
	file, err := os.CreateTemp("", "temp*")
	if err != nil {
		return nil, err
	}

	_, err = file.Write(body) // Write the byte slice to the file
	if err != nil {
		file.Close()
		return nil, err
	}

	_, err = file.Seek(0, 0) // Seek back to the beginning of the file
	if err != nil {
		file.Close()
		return nil, err
	}

	return file, err
}

func truncateUTF8Caption(s, url string) string {
	if utf8.RuneCountInString(s) <= 1017 {
		return s + fmt.Sprintf("\n<a href='%s'>🔗 Link</a>", url)
	}
	var truncated []rune
	currentLength := 0

	for _, r := range s {
		currentLength += utf8.RuneLen(r)
		if currentLength > 1017 {
			break
		}
		truncated = append(truncated, r)
	}

	return string(truncated) + "..." + fmt.Sprintf("\n<a href='%s'>🔗 Link</a>", url)
}
