package reddit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/grafov/m3u8"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var redlibInstances = []string{
	"https://redlib.catsarch.com",
	"https://red.artemislena.eu",
	"https://redlib.privacyredirect.com",
	"https://redlib.nadeko.net",
	"https://redlib.privadency.com",
	"https://redlib.4o1x5.dev",
}

var redlibRouteCounter uint64

var (
	postInfoRegex      = regexp.MustCompile(`(?:www.)?reddit.com/(?:user|r)/([^/]+)/comments/([^/]+)`)
	postTypeRegex      = regexp.MustCompile(`(?s)post_type:\s*(\w+)`)
	mediaContentRegex  = regexp.MustCompile(`(?s)<div class="post_media_content">(.*?)</div>`)
	videoRegex         = regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="video/mp4"`)
	playlistRegex      = regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="application/vnd.apple.mpegurl"`)
	imageRegex         = regexp.MustCompile(`(?s)href="([^"]+).*?class="post_media_image"`)
	thumbRegex         = regexp.MustCompile(`(?s)class="post_media_video.*?poster="([^"]+)"`)
	videoDimsRegex     = regexp.MustCompile(`(?s)<video[^>]*width="(\d+)"[^>]*height="(\d+)"`)
	galleryRegex       = regexp.MustCompile(`(?s)alt="Gallery image"\s+src="([^"]+)"`)
	cleanupRegex       = regexp.MustCompile(`(?s)(?:#\d+|amp);`)
	postAuthorRegex    = regexp.MustCompile(`(?s)<a class="post_author.*?" href=".*?">([^"]+)</a>`)
	postSubredditRegex = regexp.MustCompile(`(?s)<a class="post_subreddit" href=".*?">([^"]+)</a>`)
	postTitleRegex     = regexp.MustCompile(`(?s)<h1 class="post_title">(?:.*?</a>)?([^<]+)</h1>`)
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(fmt.Sprintf("%s/%s", handler.subreddit, handler.postID)); err == nil {
		return postInfo
	}

	medias, caption := handler.processMedia()
	if medias == nil {
		return downloader.PostInfo{}
	}

	return downloader.PostInfo{
		ID:      fmt.Sprintf("%s/%s", handler.subreddit, handler.postID),
		Medias:  medias,
		Caption: caption,
	}
}

func (h *Handler) setPostID(url string) bool {
	matches := postInfoRegex.FindStringSubmatch(url)
	if len(matches) < 3 {
		return false
	}

	h.subreddit = matches[1]
	h.postID = matches[2]
	return true
}

func (h *Handler) processMedia() ([]gotgbot.InputMedia, string) {
	medias, caption := h.getRedlibData()
	if medias != nil {
		return medias, caption
	}

	if data := h.getAPIData(); data != nil {
		medias := h.processAPIMedia(data)
		if medias == nil {
			return nil, ""
		}
		return medias, h.processAPICaption(data)
	}

	return nil, ""
}

func (h *Handler) getRedlibData() ([]gotgbot.InputMedia, string) {
	response, _, err := h.requestRedlibPost()
	if err != nil {
		return nil, ""
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, ""
	}

	postType := postTypeRegex.FindSubmatch(body)
	if len(postType) < 1 {
		return nil, ""
	}

	if string(postType[1]) == "self" {
		return nil, ""
	}

	if string(postType[1]) == "video" || string(postType[1]) == "image" {
		match := mediaContentRegex.FindSubmatch(body)
		if len(match) < 2 {
			return nil, ""
		}

		if videoMedia := h.processRedlibVideo(match[1], response); videoMedia != nil {
			return videoMedia, extractRedlibCaption(body)
		}

		if imageMedia := h.processRedlibImage(match[1], response); imageMedia != nil {
			return imageMedia, extractRedlibCaption(body)
		}
	}

	if string(postType[1]) == "gallery" {
		match := galleryRegex.FindAllSubmatch(body, -1)

		if galleryMedia := h.processRedlibGallery(match, response); galleryMedia != nil {
			return galleryMedia, extractRedlibCaption(body)
		}
	}

	return nil, ""
}

func (h *Handler) requestRedlibPost() (*http.Response, string, error) {
	startIdx := int(atomic.AddUint64(&redlibRouteCounter, 1)-1) % len(redlibInstances)

	var lastErr error
	for i := 0; i < len(redlibInstances); i++ {
		idx := (startIdx + i) % len(redlibInstances)
		instance := redlibInstances[idx]
		targetURL := fmt.Sprintf("%s/r/%s/comments/%s", instance, h.subreddit, h.postID)

		resp, err := h.fetchRedlibInstance(targetURL, instance)
		if err != nil {
			slog.Warn("Redlib instance failed, trying next", "instance", instance, "Error", err.Error())
			lastErr = err
			continue
		}
		return resp, instance, nil
	}

	return nil, "", fmt.Errorf("all redlib instances failed: last error: %w", lastErr)
}

func (h *Handler) fetchRedlibInstance(targetURL, instance string) (*http.Response, error) {
	response, err := utils.Request(targetURL, utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if response == nil || response.Body == nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, fmt.Errorf("empty response body")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	response.Body.Close()

	bodyStr := string(body)

	if looksLikeBlockPage(bodyStr) {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}
		challengeResp, err := solveAnubisChallengeForURL(targetURL, client)
		if err != nil {
			return nil, fmt.Errorf("failed to solve Anubis challenge: %w", err)
		}
		h.client = client

		body, err = io.ReadAll(challengeResp.Body)
		if err != nil {
			challengeResp.Body.Close()
			return nil, fmt.Errorf("failed to read solved response body: %w", err)
		}
		challengeResp.Body.Close()
	}

	newResp := &http.Response{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Header:     response.Header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	var reqURL *url.URL
	if response.Request != nil {
		reqURL = response.Request.URL
	}
	if reqURL == nil {
		reqURL, _ = url.Parse(instance)
	}
	newResp.Request = &http.Request{URL: reqURL}

	return newResp, nil
}

func buildMediaURL(response *http.Response, path string) string {
	raw := downloader.BuildAbsoluteURL(response.Request.URL, path)
	return cleanupRegex.ReplaceAllString(raw, "")
}

func extractRedlibCaption(body []byte) string {
	var postAuthor, postSubreddit, postTitle string
	if match := postAuthorRegex.FindSubmatch(body); len(match) > 1 {
		postAuthor = string(match[1])
	}
	if match := postSubredditRegex.FindSubmatch(body); len(match) > 1 {
		postSubreddit = string(match[1])
	}
	if match := postTitleRegex.FindSubmatch(body); len(match) > 1 {
		postTitle = string(match[1])
	}

	if postTitle != "" {
		postTitle = strings.Join(strings.Fields(postTitle), " ")
	}

	return fmt.Sprintf("<b>%s — %s</b>:\n%s",
		html.EscapeString(postAuthor),
		html.EscapeString(postSubreddit),
		html.EscapeString(postTitle))
}

func (h *Handler) processRedlibVideo(content []byte, response *http.Response) []gotgbot.InputMedia {
	playlistMatch := playlistRegex.FindSubmatch(content)
	videoMatch := videoRegex.FindSubmatch(content)

	if len(playlistMatch) < 2 {
		return nil
	}

	client := h.client
	if client == nil {
		jar, _ := cookiejar.New(nil)
		client = &http.Client{Jar: jar}
	}

	playlistURL := buildMediaURL(response, string(playlistMatch[1]))

	var videoFile []byte
	var err error

	if len(videoMatch) > 1 && string(videoMatch[1]) != "" {
		videoURL := buildMediaURL(response, string(videoMatch[1]))
		videoFile, err = downloader.FetchWithClient(videoURL, client, h.anubisSolver())
		if err != nil {
			slog.Error("Failed to download video", "Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID), "Error", err.Error())
			return nil
		}
	} else {
		videoFile, err = downloader.FetchM3U8ToBytes(playlistURL, client, h.anubisSolver())
		if err != nil {
			slog.Error("Failed to download video from HLS", "Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID), "Error", err.Error())
			return nil
		}
	}

	audioFile, err := downloadAudio(playlistURL, client, h.anubisSolver())
	if err != nil {
		slog.Error("Failed to download audio", "Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID), "Error", err.Error())
		return nil
	}

	videoFile, err = downloader.MergeAudioVideoBytes(videoFile, audioFile)
	if err != nil {
		slog.Error("Failed to merge audio and video", "Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID), "Error", err.Error())
		return nil
	}

	var width, height int64
	if dimsMatch := videoDimsRegex.FindSubmatch(content); len(dimsMatch) >= 3 {
		if w, err := strconv.Atoi(string(dimsMatch[1])); err == nil {
			width = int64(w)
		}
		if h, err := strconv.Atoi(string(dimsMatch[2])); err == nil {
			height = int64(h)
		}
	}

	thumbnail := h.downloadThumbnail(content, response)

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Reddit_%s_%s", h.subreddit, h.postID))
	videoMedia := &gotgbot.InputMediaVideo{
		Media:             downloader.InputFileFromBytes(filename, videoFile),
		SupportsStreaming: true,
		Width:             width,
		Height:            height,
	}

	if thumbnail != nil {
		videoMedia.Thumbnail = downloader.InputFileFromBytes(filename, thumbnail)
	}
	return []gotgbot.InputMedia{videoMedia}
}

func downloadAudio(playlistURL string, client *http.Client, anubisSolver downloader.AnubisSolver) ([]byte, error) {
	if playlistURL == "" {
		return nil, fmt.Errorf("empty playlist URL")
	}

	body, resp, err := downloader.FetchWithClientAndResponse(playlistURL, client, anubisSolver)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch audio playlist: %w", err)
	}

	var baseURL *url.URL
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		baseURL = resp.Request.URL
	} else {
		baseURL, _ = url.Parse(playlistURL)
	}

	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(body), true)
	if err != nil {
		return nil, fmt.Errorf("failed to decode audio playlist: %w", err)
	}

	if listType == m3u8.MASTER {
		masterPlaylist := playlist.(*m3u8.MasterPlaylist)

		audioVariant := getFirstAudioAlternative(masterPlaylist)
		if audioVariant == nil || audioVariant.URI == "" {
			return nil, fmt.Errorf("no audio variant found in master playlist")
		}

		audioPlaylistURL := downloader.BuildAbsoluteURL(baseURL, audioVariant.URI)
		return downloader.FetchM3U8ToBytes(audioPlaylistURL, client, anubisSolver)
	}

	if listType == m3u8.MEDIA {
		tmpFile, err := downloader.DownloadM3U8(bytes.NewReader(body), baseURL, client, anubisSolver)
		if err != nil {
			return nil, fmt.Errorf("failed to download audio segments: %w", err)
		}
		defer tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		audioBytes, err := io.ReadAll(tmpFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read merged audio: %w", err)
		}
		return audioBytes, nil
	}

	return nil, fmt.Errorf("unexpected playlist type for audio: %v", listType)
}

func getFirstAudioAlternative(playlist *m3u8.MasterPlaylist) *m3u8.Alternative {
	for _, variant := range playlist.Variants {
		for _, alt := range variant.Alternatives {
			if alt.Type == "AUDIO" && alt.URI != "" {
				return alt
			}
		}
	}

	for _, variant := range playlist.Variants {
		for _, alt := range variant.Alternatives {
			if alt.URI != "" {
				return alt
			}
		}
	}
	return nil
}

func (h *Handler) downloadThumbnail(content []byte, response *http.Response) []byte {
	if thumbMatch := thumbRegex.FindSubmatch(content); len(thumbMatch) > 1 {
		thumbnailURL := buildMediaURL(response, string(thumbMatch[1]))

		client := h.client
		if client == nil {
			jar, _ := cookiejar.New(nil)
			client = &http.Client{Jar: jar}
		}

		thumbnail, err := downloader.FetchWithClient(thumbnailURL, client, h.anubisSolver())
		if err != nil {
			slog.Error("Failed to download thumbnail",
				"Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID),
				"Error", err.Error())
			return nil
		}

		thumbnail, err = utils.ResizeThumbnail(thumbnail)
		if err != nil {
			slog.Error("Failed to resize thumbnail",
				"Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID),
				"Error", err.Error())
		}

		return thumbnail
	}
	return nil
}

func (h *Handler) processRedlibImage(content []byte, response *http.Response) []gotgbot.InputMedia {
	if imageMatch := imageRegex.FindSubmatch(content); len(imageMatch) > 1 {
		imageURL := buildMediaURL(response, string(imageMatch[1]))

		client := h.client
		if client == nil {
			jar, _ := cookiejar.New(nil)
			client = &http.Client{Jar: jar}
		}

		imageData, err := downloader.FetchWithClient(imageURL, client, h.anubisSolver())
		if err != nil {
			slog.Error("Failed to download image", "Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID), "Error", err.Error())
			return nil
		}

		filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Reddit_%s_%s", h.subreddit, h.postID))
		return []gotgbot.InputMedia{&gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromBytes(filename, imageData),
		}}
	}
	return nil
}

func (h *Handler) processRedlibGallery(content [][][]byte, response *http.Response) []gotgbot.InputMedia {
	if len(content) < 1 {
		return nil
	}

	type mediaResult struct {
		index int
		media gotgbot.InputMedia
		err   error
	}

	mediaCount := len(content)
	mediaItems := make([]gotgbot.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, item := range content {
		go func(index int) {
			media := buildMediaURL(response, string(item[1]))

			client := h.client
			if client == nil {
				jar, _ := cookiejar.New(nil)
				client = &http.Client{Jar: jar}
			}

			file, err := downloader.FetchWithClient(media, client, h.anubisSolver())
			if err != nil {
				results <- mediaResult{index: index, err: err}
				return
			}

			filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Reddit_%d_%s_%s", index, h.subreddit, h.postID))
			inputMedia := &gotgbot.InputMediaPhoto{
				Media: downloader.InputFileFromBytes(filename, file),
			}
			results <- mediaResult{index: index, media: inputMedia, err: nil}
		}(i)
	}

	for range mediaCount {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in gallery",
				"Post", fmt.Sprintf("%s/%s", h.subreddit, h.postID),
				"Error", result.err.Error())
			continue
		}
		mediaItems[result.index] = result.media
	}

	filtered := make([]gotgbot.InputMedia, 0, len(mediaItems))
	for _, item := range mediaItems {
		if item != nil {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (h *Handler) getAPIData() *Data {
	response, err := utils.Request(fmt.Sprintf("https://www.reddit.com/r/%s/comments/%s/.json?raw_json=1", h.subreddit, h.postID), utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})

	if err != nil || response.Body == nil || response.StatusCode != 200 {
		response, err = utils.Request(fmt.Sprintf("https://api.reddit.com/api/info/?id=t3_%s", h.postID),
			utils.RequestParams{Method: "GET",
				Headers: downloader.GenericHeaders})
		if err != nil || response.Body == nil {
			return nil
		}
		defer response.Body.Close()

		var data KindData
		err = json.NewDecoder(response.Body).Decode(&data)
		if err != nil || len(data.Data.Children) == 0 {
			return nil
		}
		return &data.Data.Children[0].Data
	}
	defer response.Body.Close()

	var data RedditPost
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil || len(data) == 0 || len(data[0].Data.Children) == 0 {
		return nil
	}

	return &data[0].Data.Children[0].Data
}

func (h *Handler) processAPIMedia(data *Data) []gotgbot.InputMedia {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Reddit_%s_%s", h.subreddit, h.postID))
	if data.IsVideo {
		video, err := downloader.FetchBytesFromURL(data.Media.RedditVideo.FallbackURL)
		if err != nil {
			slog.Error("Failed to download video",
				"Error", err.Error())
			return nil
		}

		thumbnail, err := downloader.FetchBytesFromURL(data.Preview.Images[0].Source.URL)
		if err != nil {
			slog.Error("Failed to download thumbnail",
				"Error", err.Error())
			return nil
		}

		return []gotgbot.InputMedia{&gotgbot.InputMediaVideo{
			Media:             downloader.InputFileFromBytes(filename, video),
			Width:             int64(data.Media.RedditVideo.Width),
			Height:            int64(data.Media.RedditVideo.Height),
			Thumbnail:         downloader.InputFileFromBytes(filename, thumbnail),
			SupportsStreaming: true,
		}}
	}

	if data.MediaMetadata != nil {
		type mediaResult struct {
			index int
			media gotgbot.InputMedia
			err   error
		}

		mediaCount := len(data.GalleryData.Items)
		mediaItems := make([]gotgbot.InputMedia, mediaCount)
		results := make(chan mediaResult, mediaCount)

		for i, item := range data.GalleryData.Items {
			go func(index int, mediaID string) {
				media, exists := (*data.MediaMetadata)[mediaID]
				if !exists {
					results <- mediaResult{index: index, err: fmt.Errorf("media metadata not found for media_id=%s", mediaID)}
					return
				}

				if !strings.EqualFold(media.E, "Image") {
					results <- mediaResult{index: index, media: nil, err: nil}
					return
				}

				mediaURL := normalizeRedditMediaURL(media.S.U)
				if mediaURL == "" {
					results <- mediaResult{index: index, err: fmt.Errorf("empty media url for media_id=%s", mediaID)}
					return
				}

				file, err := downloader.FetchBytesFromURL(mediaURL)
				if err != nil {
					results <- mediaResult{index: index, err: fmt.Errorf("media_id=%s url=%s: %w", mediaID, mediaURL, err)}
					return
				}

				var inputMedia gotgbot.InputMedia
				if media.E == "Image" {
					inputMedia = &gotgbot.InputMediaPhoto{
						Media: downloader.InputFileFromBytes(filename, file),
					}
				}
				results <- mediaResult{index: index, media: inputMedia, err: nil}
			}(i, item.MediaID)
		}

		for range mediaCount {
			result := <-results
			if result.err != nil {
				slog.Error("Failed to download media in gallery",
					"Error", result.err.Error())
				continue
			}
			if result.media == nil {
				continue
			}
			mediaItems[result.index] = result.media
		}

		filteredMedia := make([]gotgbot.InputMedia, 0, len(mediaItems))
		for _, media := range mediaItems {
			if media != nil {
				filteredMedia = append(filteredMedia, media)
			}
		}
		if len(filteredMedia) == 0 {
			return nil
		}

		return filteredMedia
	}

	if data.IsRedditMediaDomain && data.Domain == "i.redd.it" {
		image, err := downloader.FetchBytesFromURL(data.URL)
		if err != nil {
			slog.Error("Failed to download image",
				"Error", err.Error())
			return nil
		}

		return []gotgbot.InputMedia{&gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromBytes(filename, image),
		}}
	}

	return nil
}

func (h *Handler) processAPICaption(data *Data) string {
	return fmt.Sprintf("<b>%s — %s</b>: %s",
		html.EscapeString(data.SubredditNamePrefixed),
		html.EscapeString(data.Author),
		html.EscapeString(data.Title))
}

func normalizeRedditMediaURL(rawURL string) string {
	mediaURL := strings.TrimSpace(html.UnescapeString(rawURL))
	if mediaURL == "" {
		return ""
	}

	if strings.HasPrefix(mediaURL, "//") {
		mediaURL = "https:" + mediaURL
	}

	if strings.HasPrefix(mediaURL, "/") {
		mediaURL = "https://www.reddit.com" + mediaURL
	}

	return cleanupRegex.ReplaceAllString(mediaURL, "")
}
