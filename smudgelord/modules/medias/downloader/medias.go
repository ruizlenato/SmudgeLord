package downloader

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"unicode/utf8"

	"smudgelord/smudgelord/utils"

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

	file, err := os.CreateTemp("", fmt.Sprintf("Smudge*.%s", extension(body.Header.ContentType())))
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
