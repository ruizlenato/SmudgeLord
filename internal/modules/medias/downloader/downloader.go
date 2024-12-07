package downloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

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

func Downloader(media string) (*os.File, error) {
	response, err := utils.Request(media, utils.RequestParams{
		Method: "GET",
	})
	if err != nil || response == nil {
		return nil, errors.New("get error")
	}
	defer response.Body.Close()

	extension := func(contentType string) string {
		extension, ok := mimeExtensions[contentType]
		if !ok {
			return ""
		}
		return extension
	}

	contentDisposition := response.Header.Get("Content-Disposition")
	filename := "Smudge*." + extension(response.Header.Get("Content-Type"))

	if contentDisposition != "" {
		parts := strings.Split(contentDisposition, "filename=")
		if len(parts) > 1 {
			filename = "Smudge*" + strings.Trim(parts[1], `"`)
		}
	}

	file, err := os.CreateTemp("", filename)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()

	if _, err = io.Copy(file, response.Body); err != nil {
		return nil, err
	}

	if _, err = file.Seek(0, 0); err != nil {
		return nil, err
	}

	return file, err
}

type Medias struct {
	Caption string   `json:"caption"`
	Medias  []string `json:"medias"`
}

func SetMediaCache(replied []*telegram.NewMessage, result []string) error {
	var (
		medias  []string
		caption string
	)

	for _, message := range replied {
		if caption == "" {
			caption = utils.FormatText(message.MessageText(), message.Message.Entities)
		}

		var mediaID string

		switch m := message.Message.Media.(type) {
		case *telegram.MessageMediaPhoto:
			mediaID = telegram.PackBotFileID(m.Photo.(*telegram.PhotoObj))
		case *telegram.MessageMediaDocument:
			mediaID = telegram.PackBotFileID(m.Document.(*telegram.DocumentObj))
		}

		medias = append(medias, mediaID)
	}

	album := Medias{
		Caption: caption,
		Medias:  medias,
	}

	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+result[1], jsonValue, 48*time.Hour); err != nil {
		if !strings.Contains(err.Error(), "connect: connection refused") {
			return err
		}
	}

	return nil
}

func GetMediaCache(postID string) ([]telegram.InputMedia, string, error) {
	cached, err := cache.GetCache("media-cache:" + postID)
	if err != nil {
		return nil, "", err
	}

	var medias Medias
	if err := json.Unmarshal([]byte(cached), &medias); err != nil {
		return nil, "", fmt.Errorf("could not unmarshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+postID, cached, 48*time.Hour); err != nil {
		return nil, "", fmt.Errorf("could not reset cache expiration: %v", err)
	}

	inputMedias := make([]telegram.InputMedia, 0, len(medias.Medias))
	for _, media := range medias.Medias {
		fID, accessHash, fileType, _ := telegram.UnpackBotFileID(media)
		var inputMedia telegram.InputMedia
		switch fileType {
		case 2:
			inputMedia = &telegram.InputMediaPhoto{ID: &telegram.InputPhotoObj{ID: fID, AccessHash: accessHash}}
		case 4:
			inputMedia = &telegram.InputMediaDocument{ID: &telegram.InputDocumentObj{ID: fID, AccessHash: accessHash}}
		}
		inputMedias = append(inputMedias, inputMedia)
	}

	return inputMedias, medias.Caption, nil
}

func TruncateUTF8Caption(s, url string) string {
	if utf8.RuneCountInString(s) <= 1017 {
		return s + fmt.Sprintf("\n<a href='%s'>🔗 Link</a>", url)
	}

	truncated := make([]rune, 0, 1017)

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
	if _, err := videoFile.Seek(0, 0); err != nil {
		log.Println("[MergeAudioVideo] Error seeking video file:", err)
		return nil
	}
	if _, err := audioFile.Seek(0, 0); err != nil {
		log.Println("[MergeAudioVideo] Error seeking audio file:", err)
		return nil
	}

	defer os.Remove(videoFile.Name())
	defer os.Remove(audioFile.Name())

	outputFile, err := os.CreateTemp("", "SmudgeYoutube_*.mp4")
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
		os.Remove(outputFile.Name())
		return nil
	}

	return outputFile
}
