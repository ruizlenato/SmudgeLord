package downloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot/models"
	"github.com/grafov/m3u8"

	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func fetchURLResponse(media string) ([]byte, *http.Response, error) {
	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}
	response, err := retryCaller.Request(media, utils.RequestParams{
		Method: "GET",
	})
	if err != nil || response == nil {
		return nil, nil, errors.New("get error")
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		return nil, nil, err
	}

	return bodyBytes, response, nil
}

func FetchBytesFromURL(media string) ([]byte, error) {
	bodyBytes, response, err := fetchURLResponse(media)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if bytes.Contains(bodyBytes, []byte("#EXTM3U")) {
		tmpFile, err := downloadM3U8(bytes.NewReader(bodyBytes), response.Request.URL)
		if err != nil {
			return nil, err
		}
		defer tmpFile.Close()

		newBytes, err := io.ReadAll(tmpFile)
		if err != nil {
			return nil, err
		}
		return newBytes, nil
	}

	return bodyBytes, nil
}

func downloadM3U8(body *bytes.Reader, url *url.URL) (*os.File, error) {
	playlist, _, err := m3u8.DecodeFrom(body, true)
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
				url.Scheme,
				url.Host,
				path.Dir(url.Path),
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
	for range segmentCount {
		result := <-results
		if result.err != nil {
			slog.Error("Couldn't download segment",
				"Segment", result.index,
				"Error", result.err.Error())
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
	response, err := utils.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 5,
	})

	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	tmpFile, err := os.CreateTemp("", "*.ts")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, response.Body); err != nil {
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

func TruncateUTF8Caption(caption, url, text string, mediaCount int) string {
	const maxSizeCaption = 1024
	var textLink string
	var truncated []rune

	switch mediaCount {
	case 1:
		truncated = make([]rune, 0, 1024-3)
	default:
		textLink = fmt.Sprintf("\n<a href='%s'>🔗 %s</a>", url, text)
		truncated = make([]rune, 0, 1024-len(textLink)-3)
	}

	closingTags := extractClosingTags(caption)

	contentWithoutClosingTags := caption
	if closingTags != "" {
		contentWithoutClosingTags = caption[:len(caption)-len(closingTags)]
	}

	var currentLength int
	for _, r := range contentWithoutClosingTags {
		currentLength += utf8.RuneLen(r)
		if currentLength > 1017-len(closingTags) {
			break
		}
		truncated = append(truncated, r)
	}

	result := string(truncated) + "..." + closingTags
	if textLink != "" {
		result += textLink
	}
	return result
}

func extractClosingTags(text string) string {
	var tags []string
	remaining := text

	re := regexp.MustCompile(`(</[a-zA-Z][a-zA-Z0-9]*>)\s*$`)

	for {
		match := re.FindStringSubmatch(remaining)
		if match == nil {
			break
		}
		tags = append([]string{match[1]}, tags...)
		remaining = remaining[:len(remaining)-len(match[0])]
		remaining = strings.TrimRight(remaining, " \t\n\r")
	}

	var result strings.Builder
	for i := len(tags) - 1; i >= 0; i-- {
		result.WriteString(tags[i])
	}
	return result.String()
}

func MergeAudioVideoBytes(videoData, audioData []byte) ([]byte, error) {
	videoFile, err := os.CreateTemp("", "merge-video-*.mp4")
	if err != nil {
		return nil, err
	}
	defer func() {
		videoFile.Close()
		os.Remove(videoFile.Name())
	}()

	audioFile, err := os.CreateTemp("", "merge-audio-*.mp3")
	if err != nil {
		return nil, err
	}
	defer func() {
		audioFile.Close()
		os.Remove(audioFile.Name())
	}()

	if _, err := videoFile.Write(videoData); err != nil {
		return nil, err
	}
	if _, err := audioFile.Write(audioData); err != nil {
		return nil, err
	}

	if _, err := videoFile.Seek(0, 0); err != nil {
		return nil, err
	}
	if _, err := audioFile.Seek(0, 0); err != nil {
		return nil, err
	}

	tempOutput := videoFile.Name() + ".tmp.mp4"
	defer os.Remove(tempOutput)

	cmd := exec.Command("ffmpeg",
		"-i", videoFile.Name(),
		"-i", audioFile.Name(),
		"-c", "copy",
		"-shortest",
		"-y", tempOutput,
	)

	if err = cmd.Run(); err != nil {
		return nil, err
	}

	outFile, err := os.Open(tempOutput)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	mergedBytes, err := io.ReadAll(outFile)
	if err != nil {
		return nil, err
	}
	return mergedBytes, nil
}

func SetMediaCache(replied any, postInfo PostInfo) error {
	var (
		medias      []string
		caption     string
		invertMedia bool
	)

	var messages []*models.Message
	switch v := replied.(type) {
	case []*models.Message:
		messages = v
	case *models.Message:
		messages = []*models.Message{v}
	default:
		return errors.New("invalid type for replied")
	}

	for _, message := range messages {
		if caption == "" {
			caption = utils.FormatText(message.Caption, message.CaptionEntities)
		}
		invertMedia = message.ShowCaptionAboveMedia

		var mediaID string
		if message.Video != nil {
			mediaID = message.Video.FileID
		}
		if message.Photo != nil {
			mediaID = message.Photo[0].FileID
		}
		medias = append(medias, mediaID)
	}

	album := Medias{
		Caption:     caption,
		Medias:      medias,
		InvertMedia: invertMedia,
	}

	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("couldn't marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+postInfo.ID, jsonValue, 48*time.Hour); err != nil {
		if !strings.Contains(err.Error(), "connect: connection refused") {
			return err
		}
		return nil
	}

	return nil
}

func GetMediaCache(postID string) (PostInfo, error) {
	cached, err := cache.GetCache("media-cache:" + postID)
	if err != nil {
		return PostInfo{}, err
	}

	var medias Medias
	if err := json.Unmarshal([]byte(cached), &medias); err != nil {
		return PostInfo{}, fmt.Errorf("couldn't unmarshal medias JSON: %v", err)
	}

	inputMedias := make([]models.InputMedia, 0, len(medias.Medias))
	for _, media := range medias.Medias {
		switch utils.FileTypeByFileID(media) {
		case 2:
			inputMedias = append(inputMedias, &models.InputMediaPhoto{
				Media:                 media,
				ShowCaptionAboveMedia: medias.InvertMedia,
			})
		case 4, 9:
			inputMedias = append(inputMedias, &models.InputMediaVideo{
				Media:                 media,
				ShowCaptionAboveMedia: medias.InvertMedia,
			})
		}
	}

	return PostInfo{
		ID:          postID,
		Medias:      inputMedias,
		Caption:     medias.Caption,
		InvertMedia: medias.InvertMedia,
	}, nil
}

func SetYoutubeCache(replied *models.Message, youtubeID string) error {
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
	case "video":
		if youtube.Video == "" {
			return "", "", errors.New("video not found")
		}
		return youtube.Video, youtube.Caption, nil
	case "audio":
		if youtube.Audio == "" {
			return "", "", errors.New("audio not found")
		}
		return youtube.Audio, youtube.Caption, nil
	default:
		return "", "", nil
	}
}
