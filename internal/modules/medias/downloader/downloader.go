package downloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
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
		Method:  "GET",
		Headers: GenericHeaders,
		Cookies: map[string]string{
			"use_hls":               "on",
			"hide_hls_notification": "on",
		},
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
		tmpFile, err := DownloadM3U8(bytes.NewReader(bodyBytes), response.Request.URL, nil, nil)
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

type FetchInfo struct {
	Body        []byte
	StatusCode  int
	ContentType string
}

func FetchBytesFromURLWithClient(media string, client *http.Client) (*FetchInfo, error) {
	retryCaller := &utils.RetryCaller{
		Caller:       &utils.HTTPCaller{Client: client},
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}
	response, err := retryCaller.Request(media, utils.RequestParams{
		Method:  "GET",
		Headers: GenericHeaders,
		Cookies: map[string]string{
			"use_hls":               "on",
			"hide_hls_notification": "on",
		},
	})
	if err != nil || response == nil {
		return nil, errors.New("get error")
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	contentType := ""
	if response.Header != nil {
		contentType = response.Header.Get("Content-Type")
	}

	return &FetchInfo{
		Body:        bodyBytes,
		StatusCode:  response.StatusCode,
		ContentType: contentType,
	}, nil
}

func DownloadM3U8(body *bytes.Reader, m3u8URL *url.URL, client *http.Client, anubisSolver AnubisSolver) (*os.File, error) {
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
			urlSegment := fmt.Sprintf("%s://%s%s/%s", m3u8URL.Scheme, m3u8URL.Host, path.Dir(m3u8URL.Path), segment.URI)

			fileName, err := downloadSegment(urlSegment, client, anubisSolver)
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
			downloadErrors = append(downloadErrors, result.err)
			continue
		}
		segmentFiles[result.index] = result.fileName
	}

	if len(downloadErrors) > segmentCount/2 {
		return nil, fmt.Errorf("too many segments failed to download: %d/%d (first: %v)", len(downloadErrors), segmentCount, downloadErrors[0])
	}

	cleanSegmentFiles := make([]string, 0, len(segmentFiles))
	for _, fileName := range segmentFiles {
		if fileName != "" {
			cleanSegmentFiles = append(cleanSegmentFiles, fileName)
		}
	}

	return mergeSegments(cleanSegmentFiles)
}

type AnubisSolver func(mediaURL string, client *http.Client) ([]byte, error)

func ResolveM3U8Playlist(body []byte, baseURL *url.URL, client *http.Client, anubisSolver AnubisSolver) ([]byte, *url.URL, error) {
	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(body), true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode m3u8 playlist: %w", err)
	}

	if listType == m3u8.MEDIA {
		return body, baseURL, nil
	}

	if listType == m3u8.MASTER {
		masterPlaylist := playlist.(*m3u8.MasterPlaylist)

		var bestVariant *m3u8.Variant
		for _, v := range masterPlaylist.Variants {
			if bestVariant == nil || v.Bandwidth > bestVariant.Bandwidth {
				bestVariant = v
			}
		}
		if bestVariant == nil {
			return nil, nil, fmt.Errorf("no variant found in master playlist")
		}

		mediaPlaylistURL := BuildAbsoluteURL(baseURL, bestVariant.URI)

		mediaBody, err := FetchWithClient(mediaPlaylistURL, client, anubisSolver)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch media playlist: %w", err)
		}

		mediaBaseURL, _ := url.Parse(mediaPlaylistURL)
		return mediaBody, mediaBaseURL, nil
	}

	return nil, nil, fmt.Errorf("unexpected playlist type: %v", listType)
}

func FetchM3U8ToBytes(playlistURL string, client *http.Client, anubisSolver AnubisSolver) ([]byte, error) {
	body, resp, err := FetchWithClientAndResponse(playlistURL, client, anubisSolver)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch m3u8 playlist: %w", err)
	}

	var baseURL *url.URL
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		baseURL = resp.Request.URL
	} else {
		baseURL, _ = url.Parse(playlistURL)
	}

	mediaBody, mediaBaseURL, err := ResolveM3U8Playlist(body, baseURL, client, anubisSolver)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve m3u8 playlist: %w", err)
	}

	tmpFile, err := DownloadM3U8(bytes.NewReader(mediaBody), mediaBaseURL, client, anubisSolver)
	if err != nil {
		return nil, fmt.Errorf("failed to download m3u8 segments: %w", err)
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	result, err := io.ReadAll(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read merged m3u8 result: %w", err)
	}

	return result, nil
}

func isAnubisChallenge(body []byte) bool {
	bodyStr := string(body)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500]
	}
	lower := strings.ToLower(bodyStr)
	return strings.Contains(lower, "anubis") ||
		strings.Contains(lower, "making sure you're not a bot") ||
		strings.Contains(lower, "block_page") ||
		strings.Contains(bodyStr, "<title>403")
}

func FetchWithClient(mediaURL string, client *http.Client, anubisSolver AnubisSolver) ([]byte, error) {
	body, _, err := FetchWithClientAndResponse(mediaURL, client, anubisSolver)
	return body, err
}

func FetchWithClientAndResponse(mediaURL string, client *http.Client, anubisSolver AnubisSolver) ([]byte, *http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range GenericHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if isAnubisChallenge(body) {
		if anubisSolver != nil {
			solvedBody, solveErr := anubisSolver(mediaURL, client)
			if solveErr != nil {
				return nil, nil, solveErr
			}
			// Re-fetch to get an accurate *http.Response with correct Request.URL
			req2, err := http.NewRequest("GET", mediaURL, nil)
			if err != nil {
				return solvedBody, nil, nil
			}
			for k, v := range GenericHeaders {
				req2.Header.Set(k, v)
			}
			resp2, err := client.Do(req2)
			if err != nil {
				return solvedBody, nil, nil
			}
			return solvedBody, resp2, nil
		}
		return nil, nil, fmt.Errorf("received Anubis challenge page for %s", mediaURL)
	}

	return body, resp, nil
}

func BuildAbsoluteURL(base *url.URL, uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri
	}
	if strings.HasPrefix(uri, "/") {
		return fmt.Sprintf("%s://%s%s", base.Scheme, base.Host, uri)
	}
	return fmt.Sprintf("%s://%s%s/%s", base.Scheme, base.Host, path.Dir(base.Path), uri)
}

func downloadSegment(segmentURL string, client *http.Client, anubisSolver AnubisSolver) (string, error) {
	body, err := FetchWithClient(segmentURL, client, anubisSolver)
	if err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp("", "*.ts")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(body); err != nil {
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
	const maxQuotedTextRunes = 248
	var textLink string

	if mediaCount > 1 {
		textLink = fmt.Sprintf("\n\n<a href='%s'>🔗 %s</a>", url, text)
	}

	if len(caption)+len(textLink) <= maxSizeCaption {
		return caption + textLink
	}

	caption = limitQuotedBlockText(caption, maxQuotedTextRunes)
	if len(caption)+len(textLink) <= maxSizeCaption {
		return caption + textLink
	}

	if pre, blockquote, ok := splitCaptionByBlockquote(caption); ok {
		allowedPreBytes := maxSizeCaption - len(textLink) - len(blockquote) - len("...")
		if allowedPreBytes < 0 {
			allowedPreBytes = 0
		}

		truncatedPre := truncateUTF8ByBytes(pre, allowedPreBytes)
		candidate := utils.SanitizeTelegramHTML(truncatedPre + "..." + blockquote)
		if len(candidate)+len(textLink) <= maxSizeCaption {
			if textLink != "" {
				candidate += textLink
			}
			return candidate
		}
	}

	allowedContentBytes := maxSizeCaption - len(textLink) - len("...")
	if allowedContentBytes < 0 {
		allowedContentBytes = 0
	}

	result := utils.SanitizeTelegramHTML(truncateUTF8ByBytes(caption, allowedContentBytes) + "...")
	if textLink != "" {
		result += textLink
	}
	return result
}

func truncateUTF8ByBytes(text string, allowedBytes int) string {
	if allowedBytes <= 0 || text == "" {
		return ""
	}

	truncated := make([]rune, 0, len(text))
	currentLength := 0
	for _, r := range text {
		currentLength += utf8.RuneLen(r)
		if currentLength > allowedBytes {
			break
		}
		truncated = append(truncated, r)
	}

	return string(truncated)
}

func splitCaptionByBlockquote(caption string) (string, string, bool) {
	start := strings.Index(caption, "<blockquote>")
	end := strings.Index(caption, "</blockquote>")
	if start < 0 || end < 0 || end < start {
		return "", "", false
	}
	end += len("</blockquote>")
	return caption[:start], caption[start:end], true
}

func limitQuotedBlockText(caption string, maxRunes int) string {
	start := strings.Index(caption, "<blockquote>")
	end := strings.Index(caption, "</blockquote>")
	if start < 0 || end < 0 || end < start {
		return caption
	}

	endTagStart := end
	end += len("</blockquote>")
	block := caption[start:end]

	lineBreak := strings.Index(block, "\n")
	if lineBreak < 0 || lineBreak >= endTagStart-start {
		return caption
	}

	prefix := block[:lineBreak+1]
	quoteText := block[lineBreak+1 : endTagStart-start]
	suffix := block[endTagStart-start:]

	if utf8.RuneCountInString(quoteText) <= maxRunes {
		return caption
	}

	trimmed := []rune(quoteText)
	quoteText = string(trimmed[:maxRunes]) + "\n..."

	limitedBlock := prefix + quoteText + suffix
	return caption[:start] + limitedBlock + caption[end:]
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

func SetMediaCache(messages []gotgbot.Message, postInfo PostInfo) error {
	if len(messages) == 0 {
		return nil
	}

	var (
		medias      []string
		caption     string
		invertMedia bool
	)

	for _, message := range messages {
		if caption == "" {
			caption = sanitizeCaptionForCache(utils.FormatText(message.Caption, message.CaptionEntities))
		}
		invertMedia = invertMedia || message.ShowCaptionAboveMedia

		if len(message.Photo) > 0 {
			medias = append(medias, "p:"+message.Photo[0].FileId)
			continue
		}
		if message.Video != nil {
			medias = append(medias, "v:"+message.Video.FileId)
		}
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

var trailingOpenLinkRegex = regexp.MustCompile(`(?s)\s*<a\s+href=['"][^'"]+['"]>\s*🔗\s*[^<]*</a>\s*$`)

func sanitizeCaptionForCache(caption string) string {
	if caption == "" {
		return caption
	}

	return strings.TrimSpace(trailingOpenLinkRegex.ReplaceAllString(caption, ""))
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

	inputMedias := make([]gotgbot.InputMedia, 0, len(medias.Medias))
	for _, raw := range medias.Medias {
		mediaID := raw
		mediaType := ""

		if strings.HasPrefix(raw, "p:") {
			mediaType = "photo"
			mediaID = strings.TrimPrefix(raw, "p:")
		} else if strings.HasPrefix(raw, "v:") {
			mediaType = "video"
			mediaID = strings.TrimPrefix(raw, "v:")
		}

		if mediaType == "" {
			switch utils.FileTypeByFileID(mediaID) {
			case 2:
				mediaType = "photo"
			case 4, 9:
				mediaType = "video"
			default:
				continue
			}
		}

		switch mediaType {
		case "photo":
			inputMedias = append(inputMedias, &gotgbot.InputMediaPhoto{
				Media:                 gotgbot.InputFileByID(mediaID),
				ShowCaptionAboveMedia: medias.InvertMedia,
			})
		case "video":
			inputMedias = append(inputMedias, &gotgbot.InputMediaVideo{
				Media:                 gotgbot.InputFileByID(mediaID),
				ShowCaptionAboveMedia: medias.InvertMedia,
				SupportsStreaming:     true,
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

func SetYoutubeCache(replied *gotgbot.Message, youtubeID string) error {
	var youtube YouTube

	cached, _ := cache.GetCache("youtube-cache:" + youtubeID)
	if cached != "" {
		if err := json.Unmarshal([]byte(cached), &youtube); err != nil {
			return fmt.Errorf("couldn't unmarshal youtube JSON: %w", err)
		}
	}

	if replied == nil {
		return errors.New("youtube message is nil")
	}

	if replied.Video != nil {
		youtube = YouTube{Caption: replied.Caption, Video: replied.Video.FileId, Audio: youtube.Audio}
	} else if replied.Audio != nil {
		youtube = YouTube{Caption: replied.Caption, Video: youtube.Video, Audio: replied.Audio.FileId}
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
