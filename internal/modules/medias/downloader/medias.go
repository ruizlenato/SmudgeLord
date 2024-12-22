package downloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/grafov/m3u8"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/valyala/fasthttp"

	"github.com/mymmrac/telego"
)

type Medias struct {
	Files   []string `json:"file_id"`
	Type    []string `json:"type"`
	Caption string   `json:"caption"`
}

type YouTube struct {
	Video   string `json:"video"`
	Audio   string `json:"audio"`
	Caption string `json:"caption"`
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

func Downloader(media string) (*os.File, error) {
	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultFastHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	request, response, err := retryCaller.Request(media, utils.RequestParams{
		Method: "GET",
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response == nil {
		return nil, errors.New("get error")
	}

	if bytes.Contains(response.Body(), []byte("#EXTM3U")) {
		return downloadM3U8(request, response)
	}

	extension := func(contentType []byte) string {
		extension, ok := mimeExtensions[string(contentType)]
		if !ok {
			return ""
		}
		return extension
	}

	file, err := os.CreateTemp("", fmt.Sprintf("Smudge*.%s", extension(response.Header.ContentType())))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()

	if _, err = file.Write(response.Body()); err != nil {
		return nil, err
	}

	if _, err = file.Seek(0, 0); err != nil {
		return nil, err
	}

	return file, err
}

func downloadM3U8(request *fasthttp.Request, response *fasthttp.Response) (*os.File, error) {
	playlist, _, err := m3u8.DecodeFrom(bytes.NewReader(response.Body()), true)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode m3u8 playlist: %s", err)
	}

	mediaPlaylist := playlist.(*m3u8.MediaPlaylist)
	segmentCount := 0
	for _, segment := range mediaPlaylist.Segments {
		if segment != nil {
			segmentCount++
		}
	}

	type segmentResult struct {
		index    int
		fileName string
		err      error
	}

	results := make(chan segmentResult, segmentCount)
	segmentFiles := make([]string, segmentCount)

	for i, segment := range mediaPlaylist.Segments {
		if segment == nil {
			continue
		}

		go func(index int, segment *m3u8.MediaSegment) {
			urlSegment := fmt.Sprintf("%s://%s%s/%s",
				string(request.URI().Scheme()),
				string(request.URI().Host()),
				path.Dir(string(request.URI().Path())),
				segment.URI)

			fileName, err := downloadSegment(urlSegment)
			results <- segmentResult{
				index:    index,
				fileName: fileName,
				err:      err,
			}
		}(i, segment)
	}

	var downloadErrors []error
	for i := 0; i < segmentCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Couldn't download segment", "Segment", result.index, "Error", result.err)
			downloadErrors = append(downloadErrors, result.err)
			continue
		}
		segmentFiles[result.index] = result.fileName
	}

	if len(downloadErrors) > segmentCount/2 {
		return nil, fmt.Errorf("Too many segments failed to download: %d errors", len(downloadErrors))
	}

	cleanSegmentFiles := make([]string, 0, len(segmentFiles))
	for _, fileName := range segmentFiles {
		if fileName != "" {
			cleanSegmentFiles = append(cleanSegmentFiles, fileName)
		}
	}

	return mergeSegments(cleanSegmentFiles)
}

func downloadSegment(url string) (string, error) {
	request, response, err := utils.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 5,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp("", "SmudgeSegment-*.ts")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, bytes.NewReader(response.Body())); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func mergeSegments(segmentFiles []string) (*os.File, error) {
	listFile, err := os.CreateTemp("", "SmudgeSegment*.txt")
	if err != nil {
		return nil, err
	}
	defer listFile.Close()
	defer os.Remove(listFile.Name())

	file, err := os.CreateTemp("", "Smudge*.mp4")
	if err != nil {
		return nil, err
	}

	defer func() {
		for _, segmentFile := range segmentFiles {
			os.Remove(segmentFile)
		}
	}()

	for _, segmentFile := range segmentFiles {
		if _, err := listFile.WriteString(fmt.Sprintf("file '%s'\n", segmentFile)); err != nil {
			return nil, err
		}
	}

	cmd := exec.Command("ffmpeg", "-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy",
		file.Name())
	err = cmd.Run()
	if err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}

	return file, nil
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

func RemoveTags(text string) string {
	re := regexp.MustCompile(`(?m)^#.*`)
	cleanText := re.ReplaceAllString(text, "")
	return cleanText
}

func MergeAudioVideo(videoFile, audioFile *os.File) *os.File {
	videoFile.Seek(0, 0)
	audioFile.Seek(0, 0)

	defer os.Remove(videoFile.Name())
	defer os.Remove(audioFile.Name())

	outputFile, err := os.CreateTemp("", "SmudgeYoutube_*.mp4")
	if err != nil {
		slog.Error("Could't create temporary file", "Error", err.Error())
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
		slog.Error("Couldn't merge audio and video", "Error", err.Error())
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
			removeMediaFile(media)
		}(media)
	}

	wg.Wait()
}

func removeMediaFile(media telego.InputMedia) {
	switch media.MediaType() {
	case "photo":
		if photo, ok := media.(*telego.InputMediaPhoto); ok {
			os.Remove(photo.Media.String())
		}
	case "video":
		if video, ok := media.(*telego.InputMediaVideo); ok {
			os.Remove(video.Media.String())
			if video.Thumbnail != nil && video.Thumbnail.File.(*os.File) != nil {
				os.Remove(video.Thumbnail.String())
			}
		}
	}
}

func SetMediaCache(replied []telego.Message, result []string) error {
	files, mediasType := extractMediaInfo(replied)

	album := Medias{Caption: result[0], Files: files, Type: mediasType}
	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("Couldn't marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+result[1], jsonValue, 48*time.Hour); err != nil {
		if !strings.Contains(err.Error(), "connect: connection refused") {
			return err
		}
		return nil
	}

	return nil
}

func extractMediaInfo(replied []telego.Message) ([]string, []string) {
	var files, mediasType []string

	for _, message := range replied {
		if message.Video != nil {
			files = append(files, message.Video.FileID)
			mediasType = append(mediasType, telego.MediaTypeVideo)
		} else if message.Photo != nil {
			files = append(files, message.Photo[0].FileID)
			mediasType = append(mediasType, telego.MediaTypePhoto)
		}
	}

	return files, mediasType
}

func GetMediaCache(postID string) ([]telego.InputMedia, string, error) {
	cached, err := cache.GetCache("media-cache:" + postID)
	if err != nil {
		return nil, "", err
	}

	var medias Medias
	if err := json.Unmarshal([]byte(cached), &medias); err != nil {
		return nil, "", fmt.Errorf("Couldn't unmarshal medias JSON: %v", err)
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

func SetYoutubeCache(replied *telego.Message, youtubeID string) error {
	var youtube YouTube

	cached, _ := cache.GetCache("youtube-cache:" + youtubeID)
	if cached != "" {
		if err := json.Unmarshal([]byte(cached), &youtube); err != nil {
			return fmt.Errorf("Couldn't unmarshal youtube JSON: %w", err)
		}
	}

	if replied.Video != nil {
		youtube = YouTube{Caption: replied.Caption, Video: replied.Video.FileID, Audio: youtube.Audio}
	} else if replied.Audio != nil {
		youtube = YouTube{Caption: replied.Caption, Video: youtube.Video, Audio: replied.Audio.FileID}
	}

	jsonValue, err := json.Marshal(youtube)
	if err != nil {
		return fmt.Errorf("Couldn't marshal youtube JSON: %w", err)
	}

	if err := cache.SetCache("youtube-cache:"+youtubeID, jsonValue, 168*time.Hour); err != nil {
		return fmt.Errorf("Couldn't set youtube cache: %w", err)
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
		return "", "", fmt.Errorf("Couldn't unmarshal youtube JSON: %v", err)
	}

	if err := cache.SetCache("youtube-cache:"+youtubeID, cached, 168*time.Hour); err != nil {
		return "", "", fmt.Errorf("Couldn't reset cache expiration: %v", err)
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
