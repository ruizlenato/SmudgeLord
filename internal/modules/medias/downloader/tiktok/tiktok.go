package tiktok

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var (
	challengeScriptRegex = regexp.MustCompile(`(?s)<script[^>]*id="cs"[^>]*class="([^"]+)"[^>]*>`)
	challengeCookieRegex = regexp.MustCompile(`(?s)<script[^>]*id="wci"[^>]*class="([^"]+)"[^>]*>`)
)

func getRandomDeviceID() string {
	const minID, maxID = 7250000000000000000, 7325099899999994577
	return strconv.FormatInt(minID+rand.Int63n(maxID-minID), 10)
}

func generateRandomHex(length int) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		result[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(result)
}

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(handler.postID); err == nil {
		return postInfo
	}

	if tikTokData := handler.getTikTokData(); tikTokData != nil {
		handler.username = *tikTokData.AwemeList[0].Author.Nickname

		postInfo := downloader.PostInfo{
			ID:      handler.postID,
			Caption: getCaption(tikTokData),
		}

		if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
			postInfo.Medias = handler.handleImages(tikTokData)
		} else {
			postInfo.Medias = handler.handleVideo(tikTokData)
		}
		return postInfo
	}

	slog.Debug("TikTok: All extraction methods failed")
	return downloader.PostInfo{}
}

func (h *Handler) setPostID(url string) bool {
	postIDRegex := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`)
	if matches := postIDRegex.FindStringSubmatch(url); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	response, err := retryCaller.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})
	if err != nil {
		return false
	}
	defer response.Body.Close()

	if matches := postIDRegex.FindStringSubmatch(response.Request.URL.String()); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}
	return false
}

func extractChallengeFromHTML(body []byte) (challengeB64, cookieName string, ok bool) {
	csMatch := challengeScriptRegex.FindSubmatch(body)
	wciMatch := challengeCookieRegex.FindSubmatch(body)
	if len(csMatch) < 2 || len(wciMatch) < 2 {
		return "", "", false
	}
	return string(csMatch[1]), string(wciMatch[1]), true
}

func solveChallenge(challengeB64 string) (string, error) {
	const maxIndex = 1_000_000

	rawPayload, err := base64.StdEncoding.DecodeString(challengeB64)
	if err != nil {
		return "", err
	}

	var payload map[string]any
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return "", err
	}

	vRaw, ok := payload["v"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("missing challenge payload")
	}

	expectedDigestB64, _ := vRaw["c"].(string)
	baseHashB64, _ := vRaw["a"].(string)

	expectedDigest, _ := base64.StdEncoding.DecodeString(expectedDigestB64)
	baseHash, _ := base64.StdEncoding.DecodeString(baseHashB64)

	if len(expectedDigest) == 0 || len(baseHash) == 0 {
		return "", fmt.Errorf("invalid challenge data")
	}

	buffer := make([]byte, len(baseHash)+7)
	copy(buffer, baseHash)

	for i := 0; i <= maxIndex; i++ {
		suffix := strconv.Itoa(i)
		copy(buffer[len(baseHash):], suffix)
		candidate := buffer[:len(baseHash)+len(suffix)]

		sum := sha256.Sum256(candidate)
		if bytes.Equal(sum[:], expectedDigest) {
			payload["d"] = base64.StdEncoding.EncodeToString([]byte(suffix))
			updatedPayload, _ := json.Marshal(payload)
			return base64.StdEncoding.EncodeToString(updatedPayload), nil
		}
	}

	return "", fmt.Errorf("challenge solution not found")
}

func (h *Handler) getTikTokData() TikTokData {
	if data := h.fetchTikTokData(""); data != nil {
		return data
	}

	if h.webData == nil {
		slog.Debug("TikTok: Trying web scraping")
		h.scrapeWebData()
	}

	return h.webData
}

func (h *Handler) scrapeWebData() {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:       jar,
		Transport: &http.Transport{MaxConnsPerHost: 10},
	}

	webURL := fmt.Sprintf("https://www.tiktok.com/@_/video/%s", h.postID)

	req, _ := http.NewRequest("GET", webURL, nil)
	req.Header.Set("User-Agent", WebUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", "https://www.tiktok.com/")

	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	h.cookies = buildCookieString(jar.Cookies(resp.Request.URL))
	h.webURL = webURL

	if data := h.extractFromWebData(bodyBytes); data != nil {
		h.webData = data
		return
	}

	challengePayload, cookieName, ok := extractChallengeFromHTML(bodyBytes)
	if !ok {
		return
	}

	cookieValue, err := solveChallenge(challengePayload)
	if err != nil {
		return
	}

	challengeCookie := &http.Cookie{
		Name:   cookieName,
		Value:  cookieValue,
		Domain: ".tiktok.com",
		Path:   "/",
	}
	jar.SetCookies(resp.Request.URL, []*http.Cookie{challengeCookie})

	req2, _ := http.NewRequest("GET", webURL, nil)
	req2.Header.Set("User-Agent", WebUserAgent)
	req2.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req2.Header.Set("Referer", "https://www.tiktok.com/")

	resp2, err := client.Do(req2)
	if err != nil || resp2 == nil {
		return
	}
	resp2.Body.Close()

	h.cookies = buildCookieString(jar.Cookies(resp.Request.URL))
}

func (h *Handler) extractFromWebData(body []byte) TikTokData {
	univRegex := regexp.MustCompile(`<script[^>]*id="__UNIVERSAL_DATA_FOR_REHYDRATION__"[^>]*>([\s\S]*?)</script>`)
	match := univRegex.FindSubmatch(body)
	if len(match) < 2 {
		return nil
	}

	var webData WebUniversalData
	if err := json.Unmarshal(match[1], &webData); err != nil {
		return nil
	}

	item := webData.DefaultScope.Webapp.ItemInfo.ItemStruct
	return h.convertWebData(item)
}

func (h *Handler) convertWebData(item WebItemStruct) TikTokData {
	awemeType := 0
	if item.ImagePost != nil {
		awemeType = 2
	}

	playAddr := extractVideoURLs(item.Video)

	cover := extractCoverURLs(item.Video)

	aweme := Aweme{
		AwemeID:   item.ID,
		Desc:      &item.Desc,
		AwemeType: awemeType,
		Author: Author{
			Nickname: &item.Author.Nickname,
			UniqueID: item.Author.UniqueID,
		},
		Video: Video{
			PlayAddr:    playAddr,
			Cover:       cover,
			Duration:    item.Video.Duration,
			Width:       item.Video.Width,
			Height:      item.Video.Height,
			BitrateInfo: convertBitrateInfo(item.Video.BitrateInfo),
		},
	}

	return TikTokData(&struct {
		AwemeList []Aweme `json:"aweme_list"`
	}{AwemeList: []Aweme{aweme}})
}

func extractVideoURLs(video WebVideo) PlayAddr {
	urls := []string{}

	for _, br := range video.BitrateInfo {
		for _, url := range br.PlayAddr.URLList {
			if url != "" && !strings.Contains(url, "www.tiktok.com") {
				urls = append(urls, url)
			}
		}
	}

	if video.PlayAddr != "" && !strings.Contains(video.PlayAddr, "www.tiktok.com") {
		urls = append(urls, video.PlayAddr)
	}

	if video.DownloadAddr != "" && !strings.Contains(video.DownloadAddr, "www.tiktok.com") {
		urls = append(urls, video.DownloadAddr)
	}

	return PlayAddr{
		URLList: urls,
		Width:   video.Width,
		Height:  video.Height,
	}
}

func extractCoverURLs(video WebVideo) Cover {
	urls := []string{}

	switch v := video.Cover.(type) {
	case string:
		if v != "" {
			urls = append(urls, v)
		}
	case []any:
		if len(v) > 0 {
			if obj, ok := v[0].(map[string]any); ok {
				if urlList, ok := obj["urlList"].([]any); ok {
					for _, u := range urlList {
						if urlStr, ok := u.(string); ok && urlStr != "" {
							urls = append(urls, urlStr)
						}
					}
				}
			}
		}
	}

	if len(urls) == 0 && video.OriginCover != "" {
		urls = append(urls, video.OriginCover)
	}

	return Cover{URLList: urls}
}

func convertBitrateInfo(webBitrateInfo []WebBitrateInfo) []BitrateInfo {
	result := make([]BitrateInfo, 0, len(webBitrateInfo))

	for _, br := range webBitrateInfo {
		if len(br.PlayAddr.URLList) > 0 {
			result = append(result, BitrateInfo{
				GearName: br.GearName,
				Bitrate:  br.Bitrate,
				PlayAddr: PlayAddr{URLList: br.PlayAddr.URLList},
			})
		}
	}

	return result
}

func buildCookieString(cookies []*http.Cookie) string {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

func (h *Handler) fetchTikTokData(challengeCookie string) TikTokData {
	cookie := fmt.Sprintf("odin_tt=%s", generateRandomHex(160))
	if challengeCookie != "" {
		cookie += "; " + challengeCookie
	}

	headers := map[string]string{
		"User-Agent": AppUserAgent,
		"Accept":     "application/json",
		"Referer":    "https://www.tiktok.com/",
		"Origin":     "https://www.tiktok.com",
		"Cookie":     cookie,
	}

	queryParams := map[string]string{
		"device_platform":       "android",
		"os":                    "android",
		"ssmix":                 "a",
		"channel":               "googleplay",
		"aid":                   "0",
		"app_name":              "musical_ly",
		"version_code":          "350103",
		"version_name":          "35.1.3",
		"manifest_version_code": "2023501030",
		"update_version_code":   "2023501030",
		"device_type":           "Pixel 7",
		"device_brand":          "Google",
		"resolution":            "1080*2400",
		"dpi":                   "420",
		"language":              "en",
		"os_api":                "29",
		"os_version":            "13",
		"ac":                    "wifi",
		"timezone_name":         "America/New_York",
		"timezone_offset":       "-14400",
		"region":                "US",
		"device_id":             getRandomDeviceID(),
		"aweme_id":              h.postID,
	}

	url := fmt.Sprintf("https://%s/aweme/v1/feed/", APIHostname)

	response, err := utils.Request(url, utils.RequestParams{
		Method:  "GET",
		Headers: headers,
		Query:   queryParams,
	})

	if err != nil || response == nil || response.Body == nil {
		return nil
	}

	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	if response.StatusCode == 429 || bytes.Contains(bodyBytes, []byte("ratelimit")) {
		slog.Debug("TikTok: Rate limited")
		return nil
	}

	var tikTokData TikTokData
	if err := json.Unmarshal(bodyBytes, &tikTokData); err != nil {
		return nil
	}

	if len(tikTokData.AwemeList) == 0 || tikTokData.AwemeList[0].AwemeID != h.postID {
		return nil
	}

	return tikTokData
}

func getCaption(tikTokData TikTokData) string {
	if len(tikTokData.AwemeList) == 0 {
		return ""
	}
	aweme := tikTokData.AwemeList[0]
	if aweme.Author.Nickname != nil && aweme.Desc != nil {
		return fmt.Sprintf("<b>%s</b>:\n%s",
			html.EscapeString(*aweme.Author.Nickname),
			html.EscapeString(*aweme.Desc))
	}
	return ""
}

func (h *Handler) handleImages(tikTokData TikTokData) []models.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
		err   error
	}

	images := tikTokData.AwemeList[0].ImagePostInfo.Images
	mediaItems := make([]models.InputMedia, len(images))
	results := make(chan mediaResult, len(images))

	for i, media := range images {
		go func(index int, media Image) {
			file, err := downloader.FetchBytesFromURL(media.DisplayImage.URLList[1])
			results <- mediaResult{index, file, err}
		}(i, media)
	}

	for range images {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media", "Post", []string{h.username, h.postID}, "Index", result.index)
			continue
		}
		if result.file != nil {
			mediaItems[result.index] = &models.InputMediaPhoto{
				Media: "attach://" + utils.SanitizeString(
					fmt.Sprintf("SmudgeLord-TikTok_%d_%s_%s", result.index, h.username, h.postID)),
				MediaAttachment: bytes.NewBuffer(result.file),
			}
		}
	}

	return mediaItems
}

func (h *Handler) handleVideo(tikTokData TikTokData) []models.InputMedia {
	if len(tikTokData.AwemeList) == 0 {
		return nil
	}

	video := tikTokData.AwemeList[0].Video
	videoURLs := video.PlayAddr.URLList

	for _, br := range video.BitrateInfo {
		videoURLs = append(videoURLs, br.PlayAddr.URLList...)
	}

	var validURLs []string
	for _, url := range videoURLs {
		if url != "" && !strings.Contains(url, "www.tiktok.com") {
			validURLs = append(validURLs, url)
		}
	}

	if len(validURLs) == 0 {
		slog.Error("TikTok: No valid video URL found", "Post", h.postID)
		return nil
	}

	referer := h.webURL
	if referer == "" {
		referer = fmt.Sprintf("https://www.tiktok.com/@_/video/%s", h.postID)
	}

	var file []byte
	var err error
	for _, videoURL := range validURLs {
		file, err = h.fetchWithReferer(videoURL, referer)
		if err == nil {
			break
		}
	}

	if err != nil {
		slog.Error("TikTok: All video URLs failed", "Post", h.postID)
		return nil
	}

	var thumbnail []byte
	if len(video.Cover.URLList) > 0 {
		thumbnail, _ = h.fetchWithReferer(video.Cover.URLList[0], referer)
		if len(thumbnail) > 0 {
			thumbnail, _ = utils.ResizeThumbnail(thumbnail)
		}
	}

	var thumbnailUpload *models.InputFileUpload
	if len(thumbnail) > 0 {
		thumbnailUpload = &models.InputFileUpload{
			Filename: "thumb.jpg",
			Data:     bytes.NewBuffer(thumbnail),
		}
	}

	return []models.InputMedia{&models.InputMediaVideo{
		Media: "attach://" + utils.SanitizeString(
			fmt.Sprintf("SmudgeLord-TikTok_%s_%s", h.username, h.postID)),
		Thumbnail:         thumbnailUpload,
		Width:             video.PlayAddr.Width,
		Height:            video.PlayAddr.Height,
		Duration:          video.Duration,
		SupportsStreaming: true,
		MediaAttachment:   bytes.NewBuffer(file),
	}}
}

func (h *Handler) fetchWithReferer(url, referer string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", WebUserAgent)
	req.Header.Set("Accept", "video/webm,video/*;q=0.9,*/*;q=0.5")
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", "https://www.tiktok.com")

	if h.cookies != "" {
		req.Header.Set("Cookie", h.cookies)
	}

	client := &http.Client{
		Transport: &http.Transport{MaxConnsPerHost: 10},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 && h.cookies != "" {
				req.Header.Set("Cookie", h.cookies)
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
