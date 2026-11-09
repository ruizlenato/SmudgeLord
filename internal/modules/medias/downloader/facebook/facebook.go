// Package facebook implements a Facebook media downloader.
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

	marketplaceIDRegex            = regexp.MustCompile(`facebook\.com/marketplace/item/(\d+)`)
	marketplaceHeroTitleRegex     = regexp.MustCompile(`"marketplace_listing_title":"([^"]*)"`)
	marketplacePriceRegex         = regexp.MustCompile(`"formatted_price":\{"text":"([^"]*)"`)
	marketplacePriceFallbackRegex = regexp.MustCompile(`"listing_price":\{"amount":"([0-9.,]+)"`)
	marketplaceDescriptionRegex   = regexp.MustCompile(`"redacted_description":\{"text":"([^"]*)"`)
	marketplacePhotosArrayRegex   = regexp.MustCompile(`"listing_photos":\[([^\]]*)\]`)
	marketplacePhotoURIRegex      = regexp.MustCompile(`"uri":"([^"]*)"`)
	primaryPhotoRegex             = regexp.MustCompile(`"primary_listing_photo":\{"image":\{"uri":"([^"]*)"`)
	marketplaceFormattedAmountRegex = regexp.MustCompile(`"listing_price":\{"formatted_amount_zeros_stripped":"([^"]*)"`)
	marketplaceOffsetCurrencyRegex  = regexp.MustCompile(`"listing_price":\{"amount_with_offset_in_currency":"[^"]*","amount":"([0-9.]+)","currency":"([A-Z]{3})"`)
	marketplacePriceOffsetRegex   = regexp.MustCompile(`"listing_price":\{"amount_with_offset":"[^"]*","currency":"([A-Z]{3})","amount":"([0-9.]+)"`)
	marketplaceSellerRegex        = regexp.MustCompile(`"marketplace_listing_seller":\{(.*?)\}`)
	marketplaceSellerNameRegex    = regexp.MustCompile(`"name":"([^"]*)"`)
	shareGenericRegex             = regexp.MustCompile(`facebook\.com/share/([a-zA-Z0-9_-]+)`)

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

const (
	marketplacePriceCustomEmojiID  = "5409048419211682843"
	marketplaceSellerCustomEmojiID = "5391112412445288650"
)

func Handle(text string, _ downloader.Options) downloader.PostInfo {
	if m := marketplaceIDRegex.FindStringSubmatch(text); len(m) >= 2 {
		return handleMarketplaceItem(m[1])
	}

	if m := sharePRegex.FindStringSubmatch(text); len(m) >= 2 {
		return handleShareP(m[1])
	}

	videoID := extractVideoID(text)
	if videoID == "" {
		if m := shareGenericRegex.FindStringSubmatch(text); len(m) >= 2 {
			if id := marketplaceIDFromShareCode(m[1]); id != "" {
				return handleMarketplaceItem(id)
			}
		}
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

func handleMarketplaceItem(itemID string) downloader.PostInfo {
	cacheID := "marketplace-" + itemID
	if postInfo, err := downloader.GetMediaCache(cacheID); err == nil {
		return postInfo
	}

	htmlContent, err := fetchPage(fmt.Sprintf("https://www.facebook.com/marketplace/item/%s/", itemID))
	if err != nil {
		slog.Debug("Facebook: failed to fetch marketplace item", "ID", itemID, "Error", err)
		return downloader.NewUnavailablePostInfo(cacheID)
	}

	isRich := marketplacePhotosArrayRegex.FindStringSubmatch(htmlContent) != nil
	if !isRich {
		for attempt := 0; attempt < 2 && !isRich; attempt++ {
			richHTML, fetchErr := fetchPage(fmt.Sprintf("https://www.facebook.com/marketplace/item/%s/", itemID))
			if fetchErr == nil && marketplacePhotosArrayRegex.FindStringSubmatch(richHTML) != nil {
				htmlContent = richHTML
				isRich = true
			}
		}
	}

	if postInfo, ok := tryVideoFromHTML(htmlContent, cacheID); ok {
		return postInfo
	}

	photoURLs := extractMarketplacePhotoURLs(htmlContent)
	if len(photoURLs) == 0 {
		slog.Debug("Facebook: no photos found in marketplace item", "ID", itemID)
		return downloader.NewUnavailablePostInfo(cacheID)
	}

	var medias []gotgbot.InputMedia
	for i, photoURL := range photoURLs {
		photoBytes, err := downloadFile(photoURL)
		if err != nil {
			slog.Debug("Facebook: failed to download marketplace photo", "ID", itemID, "Index", i, "Error", err)
			continue
		}
		filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Facebook_%s_%d", cacheID, i))
		medias = append(medias, &gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromBytes(filename, photoBytes),
		})
	}

	if len(medias) == 0 {
		return downloader.NewUnavailablePostInfo(cacheID)
	}

	title, price, seller, description := extractMarketplaceListingInfo(htmlContent)

	return downloader.PostInfo{
		ID:      cacheID,
		Caption: buildMarketplaceCaption(title, price, seller, description),
		Medias:  medias,
	}
}

func extractMarketplacePhotoURLs(htmlContent string) []string {
	var photoURLs []string
	seen := map[string]bool{}

	if m := marketplacePhotosArrayRegex.FindStringSubmatch(htmlContent); len(m) >= 2 {
		for _, u := range marketplacePhotoURIRegex.FindAllStringSubmatch(m[1], -1) {
			uri := strings.ReplaceAll(u[1], `\/`, `/`)
			if uri == "" || seen[uri] {
				continue
			}
			seen[uri] = true
			photoURLs = append(photoURLs, uri)
		}
	}

	if len(photoURLs) == 0 {
		if uri := extractEscapedJSONString(htmlContent, primaryPhotoRegex); uri != "" {
			photoURLs = append(photoURLs, uri)
		}
	}

	if len(photoURLs) == 0 {
		if og := html.UnescapeString(extractMetaContent(htmlContent, ogImageRegex)); strings.Contains(og, "scontent") {
			photoURLs = append(photoURLs, og)
		}
	}

	return photoURLs
}

func extractMarketplaceListingInfo(htmlContent string) (title, price, seller, description string) {
	hero := marketplaceHeroTitleRegex.FindStringSubmatchIndex(htmlContent)
	if hero == nil {
		title = html.UnescapeString(extractMetaContent(htmlContent, ogTitleRegex))
		description = extractMetaContent(htmlContent, ogDescRegex)
		if description == "" {
			description = extractMessageText(htmlContent)
		}
		return title, "", "", html.UnescapeString(description)
	}

	title = jsonUnescape(htmlContent[hero[2]:hero[3]])

	windowEnd := min(hero[3]+64000, len(htmlContent))
	window := htmlContent[hero[3]:windowEnd]

	price = findMarketplacePrice(htmlContent, hero[3])

	if m := marketplaceSellerRegex.FindStringSubmatch(window); len(m) >= 2 {
		if name := marketplaceSellerNameRegex.FindStringSubmatch(m[1]); len(name) >= 2 {
			seller = jsonUnescape(name[1])
		}
	}

	if m := marketplaceDescriptionRegex.FindStringSubmatch(window); len(m) >= 2 {
		description = jsonUnescape(m[1])
	} else {
		description = extractMessageText(htmlContent)
	}

	return title, price, seller, description
}

func findMarketplacePrice(htmlContent string, heroEnd int) string {
	bestValue := ""
	bestDistance := 200000

	for _, candidate := range []*regexp.Regexp{marketplaceFormattedAmountRegex, marketplaceOffsetCurrencyRegex, marketplacePriceRegex, marketplacePriceOffsetRegex, marketplacePriceFallbackRegex} {
		for _, m := range candidate.FindAllStringSubmatchIndex(htmlContent, -1) {
			distance := m[0] - heroEnd
			if distance < 0 {
				distance = -distance
			}
			if distance >= bestDistance {
				continue
			}
			bestDistance = distance
			bestValue = htmlContent[m[2]:m[3]]
			switch candidate {
			case marketplaceFormattedAmountRegex, marketplacePriceRegex:
				bestValue = jsonUnescape(bestValue)
			case marketplaceOffsetCurrencyRegex:
				bestValue = formatMarketplaceOffsetPrice(htmlContent[m[4]:m[5]], bestValue)
			default:
				bestValue = formatMarketplaceOffsetPrice("", bestValue)
			}
		}
	}

	return bestValue
}

func formatMarketplaceOffsetPrice(currency, amount string) string {
	symbols := map[string]string{
		"BRL": "R$",
		"EUR": "€",
		"USD": "$",
		"GBP": "£",
		"JPY": "¥",
		"CNY": "CN¥",
		"INR": "₹",
		"KRW": "₩",
		"CHF": "CHF",
		"CAD": "C$",
		"AUD": "A$",
		"NZD": "NZ$",
		"TRY": "₺",
		"SEK": "kr",
		"NOK": "kr",
		"DKK": "kr",
		"PLN": "zł",
		"CZK": "Kč",
		"HUF": "Ft",
		"RON": "lei",
		"UAH": "₴",
		"MXN": "$",
		"ARS": "ARS$",
		"COP": "$",
		"PEN": "S/",
		"UYU": "$U",
		"BOB": "Bs",
		"CLP": "$",
		"VND": "₫",
		"THB": "฿",
		"SGD": "S$",
		"IDR": "Rp",
		"PHP": "₱",
	}
	symbol, found := symbols[currency]
	if !found {
		symbol = currency
	}
	if trimmed, found := strings.CutSuffix(amount, ".00"); found {
		amount = trimmed
	}
	return symbol + amount
}

func buildMarketplaceCaption(title, price, seller, description string) string {
	var sb strings.Builder
	if title != "" {
		fmt.Fprintf(&sb, "<b>%s</b>", html.EscapeString(title))
	}
	if price != "" {
		fmt.Fprintf(&sb, "\n<tg-emoji emoji-id=\"%s\">💵</tg-emoji> %s", marketplacePriceCustomEmojiID, html.EscapeString(price))
	}
	if seller != "" {
		fmt.Fprintf(&sb, "\n<tg-emoji emoji-id=\"%s\">🥸</tg-emoji> %s", marketplaceSellerCustomEmojiID, html.EscapeString(seller))
	}
	if description != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("<blockquote expandable>" + html.EscapeString(description) + "</blockquote>")
	}
	return sb.String()
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

var shortLinkClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func resolveShortLink(url string) string {
	finalURL := resolveRedirectURL(url)
	if finalURL == "" {
		return ""
	}
	for _, re := range []*regexp.Regexp{reelIDRegex, watchIDRegex, videoIDRegex} {
		if m := re.FindStringSubmatch(finalURL); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func resolveRedirectURL(targetURL string) string {
	resp, err := utils.RetryWithBackoff(
		func() (*http.Response, error) { return shortLinkClient.Get(targetURL) },
		3, time.Second, 5*time.Second, 2,
	)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return ""
}

func marketplaceIDFromShareCode(shareCode string) string {
	if len(shareCode) <= 1 {
		return ""
	}
	finalURL := resolveRedirectURL(fmt.Sprintf("https://www.facebook.com/share/%s/", shareCode))
	if m := marketplaceIDRegex.FindStringSubmatch(finalURL); len(m) >= 2 {
		return m[1]
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
		req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
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
	headers := downloader.CloneHeaders(downloader.GenericHeaders)
	headers["User-Agent"] = userAgent

	response, err := downloader.DefaultRetryCaller().Request(url, utils.RequestParams{
		Method:  "GET",
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return io.ReadAll(response.Body)
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
