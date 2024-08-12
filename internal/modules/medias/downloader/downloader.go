package downloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	body := utils.Request(media, utils.RequestParams{
		Method: "GET",
	})
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

	contentDisposition := body.Header.Peek("Content-Disposition")
	filename := "Smudge*." + extension(body.Header.ContentType())

	if contentDisposition != nil {
		parts := strings.Split(string(contentDisposition), "filename=")
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

	if _, err = file.Write(body.Body()); err != nil {
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

func SetMediaCache(replied []*telegram.NewMessage, postID string) error {
	var (
		medias  []string
		caption string
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	results := make(chan string, len(replied))

	for _, message := range replied {
		if caption == "" {
			caption = utils.FormatText(message.MessageText(), message.Message.Entities)
		}
		wg.Add(1)
		go func(message *telegram.NewMessage) {
			defer wg.Done()
			var mediaID string

			switch m := message.Message.Media.(type) {
			case *telegram.MessageMediaPhoto:
				mediaID = telegram.PackBotFileID(m.Photo.(*telegram.PhotoObj))
			case *telegram.MessageMediaDocument:
				mediaID = telegram.PackBotFileID(m.Document.(*telegram.DocumentObj))
			}

			results <- mediaID
		}(message)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for mediaID := range results {
		mu.Lock()
		medias = append(medias, mediaID)
		mu.Unlock()
	}

	album := Medias{
		Caption: caption,
		Medias:  medias,
	}

	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+postID, jsonValue, 48*time.Hour); err != nil {
		return fmt.Errorf("could not set cache: %v", err)
	}

	return nil
}

func GetMediaCache(postID string) ([]telegram.InputMedia, string, error) {
	type result struct {
		inputMedias []telegram.InputMedia
		caption     string
		err         error
	}

	resultChan := make(chan result)

	go func() {
		cached, err := cache.GetCache("media-cache:" + postID)
		if err != nil {
			resultChan <- result{nil, "", err}
			return
		}

		var medias Medias
		if err := json.Unmarshal([]byte(cached), &medias); err != nil {
			resultChan <- result{nil, "", fmt.Errorf("could not unmarshal JSON: %v", err)}
			return
		}

		if err := cache.SetCache("media-cache:"+postID, cached, 48*time.Hour); err != nil {
			resultChan <- result{nil, "", fmt.Errorf("could not reset cache expiration: %v", err)}
			return
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
		resultChan <- result{inputMedias, medias.Caption, nil}
	}()

	res := <-resultChan
	return res.inputMedias, res.caption, res.err
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
