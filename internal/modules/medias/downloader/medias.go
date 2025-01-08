package downloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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

var GenericHeaders = map[string]string{
	`Accept`:             `*/*`,
	`Accept-Language`:    `en`,
	`User-Agent`:         `Mozilla/5.0 (Windows NT 10.0sec-ch-ua-platform; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36`,
	`Sec-Ch-UA`:          `Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
	`Sec-Ch-UA-Mobile`:   `?0`,
	`Sec-Ch-UA-Platform`: `Windows`,
}

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

func getFileExtension(response *fasthttp.Response, request *fasthttp.Request) string {
	if mediatype, _, err := mime.ParseMediaType(string(response.Header.ContentType())); err == nil {
		if mediatype == "text/plain" {
			if ext := strings.TrimPrefix(filepath.Ext(string(request.URI().Path())), "."); ext != "" {
				return ext
			}
		}

		if exts, err := mime.ExtensionsByType(mediatype); err == nil && len(exts) > 0 {
			return strings.TrimPrefix(exts[len(exts)-1], ".")
		}
	}

	return "tmp"
}

func Downloader(media string, filename ...string) (*os.File, error) {
	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultFastHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	request, response, err := retryCaller.Request(media, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response == nil {
		return nil, errors.New("get error")
	}

	if bytes.Contains(response.Body(), []byte("#EXTM3U")) {
		return downloadM3U8(request, response)
	}

	extension := getFileExtension(response, request)

	var file *os.File
	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()

	if len(filename) == 1 {
		file, err = os.Create(filepath.Join(os.TempDir(), utils.SanitizeString(filename[0])+"."+extension))
		if err != nil {
			file, err = os.CreateTemp("", fmt.Sprintf("SmudgeLord*.%s", extension))
		}
	} else {
		file, err = os.CreateTemp("", fmt.Sprintf("SmudgeLord*.%s", extension))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

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
		return nil, fmt.Errorf("failed to decode m3u8 playlist: %s", err)
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
		return nil, fmt.Errorf("too many segments failed to download: %d errors", len(downloadErrors))
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

	tmpFile, err := os.CreateTemp("", "*.ts")
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
	listFile, err := os.CreateTemp("", "*.txt")
	if err != nil {
		return nil, err
	}
	defer listFile.Close()
	defer os.Remove(listFile.Name())

	file, err := os.CreateTemp("", "SmudgeLord*.mp4")
	if err != nil {
		return nil, err
	}

	defer func() {
		for _, segmentFile := range segmentFiles {
			os.Remove(segmentFile)
		}
	}()

	for _, segmentFile := range segmentFiles {
		if _, err := fmt.Fprintf(listFile, "file '%s'\n", segmentFile); err != nil {
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

func MergeAudioVideo(videoFile, audioFile *os.File) (err error) {
	if _, err := videoFile.Seek(0, 0); err != nil {
		return err
	}
	if _, err := audioFile.Seek(0, 0); err != nil {
		return err
	}

	videoName := videoFile.Name()
	tempOutput := videoName + ".tmp" + filepath.Ext(videoName)

	defer func() {
		err = os.Remove(tempOutput)
		if err != nil {
			slog.Error("Couldn't remove temporary file",
				"Error", err.Error())
		}
		err = os.Remove(audioFile.Name())
		if err != nil {
			slog.Error("Couldn't remove audio file",
				"Error", err.Error())
		}
	}()

	cmd := exec.Command("ffmpeg",
		"-i", videoName,
		"-i", audioFile.Name(),
		"-c", "copy",
		"-shortest",
		"-y", tempOutput,
	)

	if err = cmd.Run(); err != nil {
		return err
	}

	tempFile, err := os.Open(tempOutput)
	if err != nil {
		return err
	}
	defer tempFile.Close()

	if _, err = io.Copy(videoFile, tempFile); err != nil {
		return err
	}

	return err
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
		return fmt.Errorf("couldn't marshal JSON: %v", err)
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
		return nil, "", fmt.Errorf("couldn't unmarshal medias JSON: %v", err)
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
			return fmt.Errorf("couldn't unmarshal youtube JSON: %w", err)
		}
	}

	if replied.Video != nil {
		youtube = YouTube{Caption: replied.Caption, Video: replied.Video.FileID, Audio: youtube.Audio}
	} else if replied.Audio != nil {
		youtube = YouTube{Caption: replied.Caption, Video: youtube.Video, Audio: replied.Audio.FileID}
	}

	jsonValue, err := json.Marshal(youtube)
	if err != nil {
		return fmt.Errorf("couldn't marshal youtube JSON: %w", err)
	}

	if err := cache.SetCache("youtube-cache:"+youtubeID, jsonValue, 168*time.Hour); err != nil {
		if !strings.Contains(err.Error(), "connect: connection refused") {
			return err
		}
		return nil
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
		return "", "", fmt.Errorf("couldn't unmarshal youtube JSON: %v", err)
	}

	if err := cache.SetCache("youtube-cache:"+youtubeID, cached, 168*time.Hour); err != nil {
		return "", "", fmt.Errorf("couldn't reset cache expiration: %v", err)
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
