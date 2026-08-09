package facebook

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var (
	reelIDRegex  = regexp.MustCompile(`facebook\.com/reel/(\d+)`)
	watchIDRegex = regexp.MustCompile(`[?&]v=(\d+)`)
	videoIDRegex = regexp.MustCompile(`/videos/(\d+)`)
	shareVRegex  = regexp.MustCompile(`facebook\.com/share/v/([a-zA-Z0-9_-]+)`)
	shareRRegex  = regexp.MustCompile(`facebook\.com/share/r/([a-zA-Z0-9_-]+)`)
	sharePRegex  = regexp.MustCompile(`facebook\.com/share/p/([a-zA-Z0-9_-]+)`)
	fbWatchRegex = regexp.MustCompile(`fb\.watch/([a-zA-Z0-9_-]+)`)

	hdURLRegex       = regexp.MustCompile(`"browser_native_hd_url":"([^"]*)"`)
	sdURLRegex       = regexp.MustCompile(`"browser_native_sd_url":"([^"]*)"`)
	playableURLRegex = regexp.MustCompile(`"playable_url":"([^"]*)"`)
	ownerNameRegex   = regexp.MustCompile(`"video_owner":\{[^}]*"name":"([^"]*)"`)
	ogTitleRegex     = regexp.MustCompile(`property=["']og:title["'][^>]+content=["']([^"']*)["']`)
	ogImageRegex     = regexp.MustCompile(`property=["']og:image["'][^>]+content=["']([^"']*)["']`)
	ogDescRegex      = regexp.MustCompile(`property=["']og:description["'][^>]+content=["']([^"']*)["']`)
	viewerImageRegex = regexp.MustCompile(`"viewer_image":\{[^}]*"uri":"([^"]*)"`)
	messageTextRegex = regexp.MustCompile(`"message":\{"__typename":"TextWithEntities","text":"([^"]*)"`)

	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

func Handle(text string, _ downloader.Options) downloader.PostInfo {
	if m := sharePRegex.FindStringSubmatch(text); len(m) >= 2 {
		return handleShareP(m[1])
	}

	videoID := extractVideoID(text)
	if videoID == "" {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(videoID); err == nil {
		return postInfo
	}

	htmlContent, err := fetchPage(fmt.Sprintf("https://www.facebook.com/reel/%s", videoID))
	if err != nil {
		slog.Debug("Facebook: failed to fetch reel page", "ID", videoID, "Error", err)
		return downloader.NewUnavailablePostInfo(videoID)
	}

	postInfo, ok := tryVideoFromHTML(htmlContent, videoID)
	if !ok {
		slog.Debug("Facebook: no video URLs found", "ID", videoID)
		return downloader.NewNoMediaPostInfo(videoID)
	}
	return postInfo
}

func handleShareP(shareCode string) downloader.PostInfo {
	if postInfo, err := downloader.GetMediaCache(shareCode); err == nil {
		return postInfo
	}

	shareURL := fmt.Sprintf("https://www.facebook.com/share/p/%s/", shareCode)
	htmlContent, err := fetchPage(shareURL)
	if err != nil {
		slog.Debug("Facebook: failed to fetch share/p page", "Code", shareCode, "Error", err)
		return downloader.NewUnavailablePostInfo(shareCode)
	}

	if postInfo, ok := tryVideoFromHTML(htmlContent, shareCode); ok {
		return postInfo
	}

	photoURLs := extractPhotoURLs(htmlContent)
	if len(photoURLs) == 0 {
		slog.Debug("Facebook: no media found in share/p", "Code", shareCode)
		return downloader.NewNoMediaPostInfo(shareCode)
	}

	var medias []gotgbot.InputMedia
	for i, photoURL := range photoURLs {
		photoBytes, err := downloadFile(photoURL)
		if err != nil {
			slog.Debug("Facebook: failed to download photo", "Code", shareCode, "Index", i, "Error", err)
			continue
		}
		filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Facebook_%s_%d", shareCode, i))
		medias = append(medias, &gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromBytes(filename, photoBytes),
		})
	}

	if len(medias) == 0 {
		return downloader.NewUnavailablePostInfo(shareCode)
	}

	authorName := extractMetaContent(htmlContent, ogTitleRegex)
	description := extractMessageText(htmlContent)
	if description == "" {
		description = extractMetaContent(htmlContent, ogDescRegex)
	}

	return downloader.PostInfo{
		ID:      shareCode,
		Caption: buildCaption(authorName, description),
		Medias:  medias,
	}
}

func tryVideoFromHTML(htmlContent, id string) (downloader.PostInfo, bool) {
	hdURL := extractEscapedJSONString(htmlContent, hdURLRegex)
	sdURL := extractEscapedJSONString(htmlContent, sdURLRegex)
	if hdURL == "" {
		hdURL = extractEscapedJSONString(htmlContent, playableURLRegex)
	}

	if hdURL == "" && sdURL == "" {
		return downloader.PostInfo{}, false
	}

	videoURL := hdURL
	if videoURL == "" {
		videoURL = sdURL
	}

	authorName := extractMetaContent(htmlContent, ownerNameRegex)
	thumbnailURL := extractMetaContent(htmlContent, ogImageRegex)
	description := extractFullDescription(extractMetaContent(htmlContent, ogTitleRegex), extractMetaContent(htmlContent, ogDescRegex))

	videoBytes, err := downloadFile(videoURL)
	if err != nil {
		slog.Error("Facebook: failed to download video", "ID", id, "Error", err)
		return downloader.NewUnavailablePostInfo(id), true
	}

	var thumbnail []byte
	if thumbnailURL != "" {
		thumbnail, err = downloadFile(thumbnailURL)
		if err == nil && len(thumbnail) > 0 {
			thumbnail, _ = utils.ResizeThumbnail(thumbnail)
		}
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Facebook_%s", id))
	media := &gotgbot.InputMediaVideo{
		Media:             downloader.InputFileFromBytes(filename, videoBytes),
		SupportsStreaming: true,
	}
	if len(thumbnail) > 0 {
		media.Thumbnail = downloader.InputFileFromBytes(filename, thumbnail)
	}

	return downloader.PostInfo{
		ID:      id,
		Caption: buildCaption(authorName, description),
		Medias:  []gotgbot.InputMedia{media},
	}, true
}

func extractVideoID(text string) string {
	for _, re := range []*regexp.Regexp{reelIDRegex, watchIDRegex, videoIDRegex} {
		if m := re.FindStringSubmatch(text); len(m) >= 2 {
			return m[1]
		}
	}
	if m := fbWatchRegex.FindStringSubmatch(text); len(m) >= 2 {
		return resolveShortLink(fmt.Sprintf("https://fb.watch/%s/", m[1]))
	}
	if m := shareVRegex.FindStringSubmatch(text); len(m) >= 2 {
		return resolveShortLink(fmt.Sprintf("https://www.facebook.com/share/v/%s/", m[1]))
	}
	if m := shareRRegex.FindStringSubmatch(text); len(m) >= 2 {
		return resolveShortLink(fmt.Sprintf("https://www.facebook.com/share/r/%s/", m[1]))
	}
	return ""
}

func resolveShortLink(url string) string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL := resp.Request.URL.String()
		for _, re := range []*regexp.Regexp{reelIDRegex, watchIDRegex, videoIDRegex} {
			if m := re.FindStringSubmatch(finalURL); len(m) >= 2 {
				return m[1]
			}
		}
	}
	return ""
}

func fetchPage(targetURL string) (string, error) {
	const maxAttempts = 3

	setBrowserHeaders := func(req *http.Request) {
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}

		req, _ := http.NewRequest("GET", "https://www.facebook.com/", nil)
		setBrowserHeaders(req)
		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("homepage request failed: %w", err)
			}
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		time.Sleep(2 * time.Second)

		req2, _ := http.NewRequest("GET", targetURL, nil)
		setBrowserHeaders(req2)
		resp2, err := client.Do(req2)
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("page request failed: %w", err)
			}
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if err != nil {
			if attempt == maxAttempts {
				return "", fmt.Errorf("failed to read page: %w", err)
			}
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		if len(body) < 5000 {
			if attempt == maxAttempts {
				return "", fmt.Errorf("page too small (%d bytes), likely blocked", len(body))
			}
			slog.Debug("Facebook: page too small, retrying", "URL", targetURL, "Attempt", attempt, "Size", len(body))
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		return string(body), nil
	}

	return "", fmt.Errorf("failed after %d attempts", maxAttempts)
}

func extractEscapedJSONString(html string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.ReplaceAll(m[1], `\/`, `/`)
}

func extractMetaContent(html string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func extractFullDescription(ogTitle, ogDescription string) string {
	if ogTitle == "" {
		return ogDescription
	}
	parts := strings.Split(ogTitle, " | ")
	var fullText string
	switch len(parts) {
	case 1:
		fullText = parts[0]
	case 2:
		fullText = parts[1]
	default:
		fullText = strings.Join(parts[1:len(parts)-1], " | ")
	}
	if len(fullText) > len(ogDescription) {
		return fullText
	}
	return ogDescription
}

func jsonUnescape(s string) string {
	var result string
	if err := json.Unmarshal([]byte(`"`+s+`"`), &result); err == nil {
		return result
	}
	return s
}

func buildCaption(title, description string) string {
	title = html.UnescapeString(jsonUnescape(title))
	description = html.UnescapeString(jsonUnescape(description))
	if title == "" && description == "" {
		return ""
	}
	if title == "" {
		return html.EscapeString(description)
	}
	if description == "" {
		return html.EscapeString(title)
	}
	return fmt.Sprintf("<b>%s</b>:\n%s", html.EscapeString(title), html.EscapeString(description))
}

func downloadFile(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func extractPhotoURLs(htmlContent string) []string {
	matches := viewerImageRegex.FindAllStringSubmatch(htmlContent, -1)
	seen := map[string]bool{}
	var urls []string
	for _, m := range matches {
		uri := strings.ReplaceAll(m[1], `\/`, `/`)
		if uri == "" || seen[uri] {
			continue
		}
		seen[uri] = true
		urls = append(urls, uri)
	}
	return urls
}

func extractMessageText(htmlContent string) string {
	m := messageTextRegex.FindStringSubmatch(htmlContent)
	if len(m) < 2 {
		return ""
	}
	return jsonUnescape(m[1])
}
