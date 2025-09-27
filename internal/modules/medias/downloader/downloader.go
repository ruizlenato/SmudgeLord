package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/grafov/m3u8"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func fetchURLResponse(media string) ([]byte, *http.Response, error) {
	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   2 * time.Second,
		MaxDelay:     10 * time.Second,
	}
	response, err := retryCaller.Request(media, utils.RequestParams{
		Method: "GET",
	})
	if err != nil || response == nil {
		return nil, nil, errors.New("get error")
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
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
		defer os.Remove(tmpFile.Name())

		newBytes, err := io.ReadAll(tmpFile)
		if err != nil {
			return nil, err
		}
		return newBytes, nil
	}

	return bodyBytes, nil
}

func SetMediaCache(replied any, postInfo PostInfo) error {
	var (
		medias      []string
		caption     string
		invertMedia bool
	)

	var messages []*telegram.NewMessage
	switch v := replied.(type) {
	case []*telegram.NewMessage:
		messages = v
	case *telegram.NewMessage:
		messages = []*telegram.NewMessage{v}
	default:
		return fmt.Errorf("unsupported type: expected []*telegram.NewMessage or *telegram.NewMessage, got %T", replied)
	}

	for _, message := range messages {
		if caption == "" {
			caption = utils.FormatText(message.MessageText(), message.Message.Entities)
		}

		invertMedia = message.Message.InvertMedia

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
		Caption:     caption,
		Medias:      medias,
		InvertMedia: invertMedia,
	}

	jsonValue, err := json.Marshal(album)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+postInfo.ID, jsonValue, 48*time.Hour); err != nil {
		if !strings.Contains(err.Error(), "connect: connection refused") {
			return err
		}
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
		return PostInfo{}, fmt.Errorf("could not unmarshal JSON: %v", err)
	}

	if err := cache.SetCache("media-cache:"+postID, cached, 48*time.Hour); err != nil {
		return PostInfo{}, fmt.Errorf("could not reset cache expiration: %v", err)
	}

	inputMedias := make([]telegram.InputMedia, 0, len(medias.Medias))
	for _, media := range medias.Medias {
		fID, accessHash, fileType, _ := telegram.UnpackBotFileID(media)
		var inputMedia telegram.InputMedia
		switch fileType {
		case 2:
			inputMedia = &telegram.InputMediaPhoto{ID: &telegram.InputPhotoObj{ID: fID, AccessHash: accessHash}}
		case 4, 9:
			inputMedia = &telegram.InputMediaDocument{ID: &telegram.InputDocumentObj{ID: fID, AccessHash: accessHash}}
		}
		inputMedias = append(inputMedias, inputMedia)
	}

	return PostInfo{
		ID:          postID,
		Medias:      inputMedias,
		Caption:     medias.Caption,
		InvertMedia: medias.InvertMedia,
	}, nil
}

func downloadM3U8(body *bytes.Reader, playlistURL *url.URL) (*os.File, error) {
	playlist, listType, err := m3u8.DecodeFrom(body, true)
	if err != nil {
		return nil, fmt.Errorf("failed to decode m3u8 playlist: %s", err)
	}

	if listType != m3u8.MEDIA {
		return nil, fmt.Errorf("playlist is not a media playlist")
	}

	mediaPlaylist := playlist.(*m3u8.MediaPlaylist)

	tmpFile, err := os.CreateTemp("", "SmudgeOutput-*.temp")
	if err != nil {
		return nil, err
	}

	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if mediaPlaylist.Map != nil && mediaPlaylist.Map.URI != "" {
		initURL := resolveURL(playlistURL, mediaPlaylist.Map.URI)

		response, err := utils.Request(initURL, utils.RequestParams{
			Method:    "GET",
			Redirects: 5,
		})
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		if _, err := io.Copy(tmpFile, response.Body); err != nil {
			return nil, fmt.Errorf("failed to write init segment: %w", err)
		}
	}

	for _, segment := range mediaPlaylist.Segments {
		if segment == nil {
			continue
		}

		segmentURL := resolveURL(playlistURL, segment.URI)

		response, err := utils.Request(segmentURL, utils.RequestParams{
			Method:    "GET",
			Redirects: 5,
		})
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		_, err = io.Copy(tmpFile, response.Body)
		if err != nil {
			return nil, err
		}
	}
	return remuxSegments(tmpFile)
}

func resolveURL(base *url.URL, ref string) string {
	parsedURI, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(parsedURI).String()
}

func remuxSegments(input *os.File) (*os.File, error) {
	defer input.Close()

	output, err := os.CreateTemp("", "Smudge*.mp4")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("ffmpeg", "-y",
		"-i", input.Name(),
		"-c", "copy",
		output.Name())

	err = cmd.Run()
	if err != nil {
		output.Close()
		os.Remove(output.Name())
		return nil, err
	}

	return output, nil
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

	var currentLength int

	for _, r := range caption {
		currentLength += utf8.RuneLen(r)
		if currentLength > 1017 {
			break
		}
		truncated = append(truncated, r)
	}

	if textLink == "" {
		return string(truncated) + "..."
	}
	return string(truncated) + "..." + textLink
}

func MergeAudioVideo(videoData, audioData []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-i", "pipe:3",
		"-c", "copy",
		"-shortest",
		"-movflags", "frag_keyframe+empty_moov",
		"-bsf:a", "aac_adtstoasc",
		"-f", "mp4",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	videoReader, videoWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create video pipe: %w", err)
	}
	defer videoReader.Close()

	audioReader, audioWriter, err := os.Pipe()
	if err != nil {
		videoWriter.Close()
		return nil, fmt.Errorf("failed to create audio pipe: %w", err)
	}
	defer audioReader.Close()

	cmd.Stdin = videoReader
	cmd.ExtraFiles = []*os.File{audioReader}

	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.Run()
	}()

	go func() {
		defer videoWriter.Close()
		videoWriter.Write(videoData)
	}()

	go func() {
		defer audioWriter.Close()
		audioWriter.Write(audioData)
	}()

	select {
	case err = <-errChan:
	case <-ctx.Done():
		return nil, fmt.Errorf("ffmpeg merge timed out after 60s: %w\nStderr: %s", ctx.Err(), stderr.String())
	}

	if err != nil {
		return nil, fmt.Errorf("ffmpeg error during merge: %w\nStderr: %s", err, stderr.String())
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output during merge\nStderr: %s", stderr.String())
	}

	return stdout.Bytes(), nil
}
