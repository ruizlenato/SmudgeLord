package medias

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	} else if match, _ := regexp.MatchString("(?:reddit|twitch).(?:com|tv)", url); match {
		dm.Generic(url)
	}

	if dm.MediaItems != nil && dm.Caption == "" {
		dm.Caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url)
	}

	if utf8.RuneCountInString(dm.Caption) > 1024 {
		dm.Caption = truncateUTF8Caption(dm.Caption, url)
	}

	return dm.MediaItems, dm.Caption
}

var mimeExtensions = map[string]string{
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"image/gif":       "gif",
	"image/webp":      "webp",
	"video/mp4":       "mp4",
	"video/webm":      "webm",
	"video/quicktime": "mov",
	"video/x-msvideo": "avi",
}

func downloader(media string) (*os.File, error) {
	body := utils.RequestGET(media, utils.RequestGETParams{})
	if body == nil {
		return nil, errors.New("get error")
	}

	extension := func(contentType []byte) string {
		extension, ok := mimeExtensions[string(contentType)]
		if !ok {
			return ""
		}
		return extension
	}

	file, err := os.CreateTemp("", fmt.Sprintf("smudge*.%s", extension(body.Header.ContentType())))
	if err != nil {
		return nil, err
	}

	// Defer a function to close and remove the file in case of error
	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()

	_, err = file.Write(body.Body()) // Write the byte slice to the file
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, 0) // Seek back to the beginning of the file
	if err != nil {
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

func MergeAudioVideo(videoFile, audioFile *os.File) *os.File {
	videoFile.Seek(0, 0)
	audioFile.Seek(0, 0)

	outputFile, err := os.CreateTemp("", "youtube_*.m4a")
	if err != nil {
		log.Println("[MergeAudioVideo] Error creating temp file:", err)
		return nil
	}

	ffmpegCMD := exec.Command("ffmpeg", "-y",
		"-loglevel", "warning",
		"-i", videoFile.Name(),
		"-i", audioFile.Name(),
		"-c", "copy", // Just copy
		"-shortest",
		outputFile.Name(),
	)

	err = ffmpegCMD.Run()
	if err != nil {
		log.Println("[MergeAudioVideo] Error running ffmpeg:", err)
		return nil
	}
	outputFile.Seek(0, 0)
	os.Remove(videoFile.Name())
	os.Remove(audioFile.Name())
	return outputFile
}
