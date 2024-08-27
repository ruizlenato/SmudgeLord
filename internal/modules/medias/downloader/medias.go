package downloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/utils"

	"github.com/mymmrac/telego"
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

	file, err := os.CreateTemp("", fmt.Sprintf("Smudge*.%s", extension(body.Header.ContentType())))
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

func TruncateUTF8Caption(s, url string) string {
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

func RemoveMediaFiles(mediaItems []telego.InputMedia) {
	var wg sync.WaitGroup

	for _, media := range mediaItems {
		wg.Add(1)

		go func(media telego.InputMedia) {
			defer wg.Done()

			switch media.MediaType() {
			case "photo":
				if photo, ok := media.(*telego.InputMediaPhoto); ok {
					os.Remove(photo.Media.String())
				}
			case "video":
				if video, ok := media.(*telego.InputMediaVideo); ok {
					os.Remove(video.Media.String())
				}
			}
		}(media)
	}

	wg.Wait()
}

type Medias struct {
	Files   []string `json:"file_id"`
	Type    []string `json:"type"`
	Caption string   `json:"caption"`
}

func SetMediaCache(replied []telego.Message, postID string) error {
	var (
		files      []string
		mediasType []string
		caption    string
		wg         sync.WaitGroup
		mu         sync.Mutex
	)

	results := make(chan struct {
		mediaType string
		fileID    string
	}, len(replied))

	for _, message := range replied {
		if caption == "" {
			caption = utils.FormatText(message.Caption, message.CaptionEntities)
		}
		wg.Add(1)
		go func(message telego.Message) {
			defer wg.Done()
			if message.Video != nil {
				results <- struct {
					mediaType string
					fileID    string
				}{telego.MediaTypeVideo, message.Video.FileID}
			} else if message.Photo != nil {
				results <- struct {
					mediaType string
					fileID    string
				}{telego.MediaTypePhoto, message.Photo[0].FileID}
			}
		}(message)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		mu.Lock()
		files = append(files, result.fileID)
		mediasType = append(mediasType, result.mediaType)
		mu.Unlock()
	}

	album := Medias{Caption: caption, Files: files, Type: mediasType}

	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+postID, jsonValue, 48*time.Hour); err != nil {
		return fmt.Errorf("could not set cache: %v", err)
	}

	return nil
}

func GetMediaCache(postID string) ([]telego.InputMedia, string, error) {
	type result struct {
		inputMedias []telego.InputMedia
		caption     string
		err         error
	}

	resultChan := make(chan result, 1)

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

		inputMedias := make([]telego.InputMedia, 0, len(medias.Files))
		for i, media := range medias.Files {
			switch medias.Type[i] {
			case telego.MediaTypeVideo:
				inputMedias = append(inputMedias, &telego.InputMediaVideo{
					Type:  telego.MediaTypeVideo,
					Media: telego.InputFile{FileID: media},
				})
			case telego.MediaTypePhoto:
				inputMedias = append(inputMedias, &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{FileID: media},
				})
			}
		}

		resultChan <- result{inputMedias, medias.Caption, nil}
	}()

	res := <-resultChan
	return res.inputMedias, res.caption, res.err
}
