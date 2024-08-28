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
		"-c", "copy",
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

func SetMediaCache(replied []telego.Message, result []string) error {
	var (
		files      []string
		mediasType []string
	)

	for _, message := range replied {
		if message.Video != nil {
			files = append(files, message.Video.FileID)
			mediasType = append(mediasType, telego.MediaTypeVideo)
		} else if message.Photo != nil {
			files = append(files, message.Photo[0].FileID)
			mediasType = append(mediasType, telego.MediaTypePhoto)
		}
	}

	album := Medias{Caption: result[0], Files: files, Type: mediasType}
	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+result[1], jsonValue, 48*time.Hour); err != nil {
		return fmt.Errorf("could not set cache: %v", err)
	}

	return nil
}

func GetMediaCache(postID string) ([]telego.InputMedia, string, error) {
	cached, err := cache.GetCache("media-cache:" + postID)
	if err != nil {
		return nil, "", err
	}

	var medias Medias
	if err := json.Unmarshal([]byte(cached), &medias); err != nil {
		return nil, "", fmt.Errorf("could not unmarshal medias JSON: %v", err)
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

	return inputMedias, medias.Caption, nil
}

type YouTube struct {
	Video   string `json:"video"`
	Audio   string `json:"audio"`
	Caption string `json:"caption"`
}

func SetYoutubeCache(replied *telego.Message, youtubeID string) error {
	var youtube YouTube

	cached, _ := cache.GetCache("youtube-cache:" + youtubeID)
	if cached != "" {
		if err := json.Unmarshal([]byte(cached), &youtube); err != nil {
			return fmt.Errorf("could not unmarshal youtube JSON: %w", err)
		}
	}

	if replied.Video != nil {
		youtube = YouTube{Caption: replied.Caption, Video: replied.Video.FileID, Audio: youtube.Audio}
	} else if replied.Audio != nil {
		youtube = YouTube{Caption: replied.Caption, Video: youtube.Video, Audio: replied.Audio.FileID}
	}

	jsonValue, err := json.Marshal(youtube)
	if err != nil {
		return fmt.Errorf("could not marshal youtube JSON: %w", err)
	}

	if err := cache.SetCache("youtube-cache:"+youtubeID, jsonValue, 168*time.Hour); err != nil {
		return fmt.Errorf("could not set youtube cache: %w", err)
	}

	return nil
}

func GetYoutubeCache(youtubeID string, format string) (string, string, error) {
	cached, err := cache.GetCache("youtube-cache:" + youtubeID)
	if err != nil {
		return "", "", err
	}

	var youtube YouTube
	if err := json.Unmarshal([]byte(cached), &youtube); err != nil {
		return "", "", fmt.Errorf("could not unmarshal youtube JSON: %v", err)
	}

	if err := cache.SetCache("youtube-cache:"+youtubeID, cached, 168*time.Hour); err != nil {
		return "", "", fmt.Errorf("could not reset cache expiration: %v", err)
	}

	switch format {
	case telego.MediaTypeVideo:
		if youtube.Video == "" {
			return "", "", errors.New("video not found")
		}
		return youtube.Video, youtube.Caption, nil
	case telego.MediaTypeAudio:
		if youtube.Audio == "" {
			return "", "", errors.New("audio not found")
		}
		return youtube.Audio, youtube.Caption, nil
	default:
		return "", "", nil
	}
}
