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
	"path"
	"regexp"
	"strings"

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
	postInfoRegex     = regexp.MustCompile(`(?:www.)?reddit.com/(?:user|r)/([^/]+)/comments/([^/]+)`)
	postTypeRegex     = regexp.MustCompile(`(?s)post_type:\s*(\w+)`)
	mediaContentRegex = regexp.MustCompile(`(?s)<div class="post_media_content">(.*?)</div>`)
	videoRegex        = regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="video/mp4"`)
	playlistRegex     = regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="application/vnd.apple.mpegurl"`)
	imageRegex        = regexp.MustCompile(`(?s)href="([^"]+).*?class="post_media_image"`)
	thumbRegex        = regexp.MustCompile(`(?s)class="post_media_video.*?poster="([^"]+)"`)
	galleryRegex      = regexp.MustCompile(`(?s)alt="Gallery image"\s+src="([^"]+)"`)
	cleanupRegex      = regexp.MustCompile(`(?s)(?:#\d+|amp);`)
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
	if len(postType) < 1 || string(postType[1]) == "self" {
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
	instance := "https://redlib.catsarch.com"
	targetURL := fmt.Sprintf("%s/r/%s/comments/%s", instance, h.subreddit, h.postID)

	response, err := utils.Request(targetURL, utils.RequestParams{
		Method:  "GET",
		Headers: downloader.GenericHeaders,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to request instance %s: %w", instance, err)
	}

	if response == nil || response.Body == nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, "", fmt.Errorf("empty response body from instance %s", instance)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}
	response.Body.Close()

	bodyStr := string(body)

	if looksLikeBlockPage(bodyStr) {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}
		challengeResp, err := solveAnubisChallengeForURL(targetURL, client)
		if err != nil {
			return nil, "", fmt.Errorf("failed to solve Anubis challenge: %w", err)
		}
		h.client = client

		body, err = io.ReadAll(challengeResp.Body)
		if err != nil {
			challengeResp.Body.Close()
			return nil, "", fmt.Errorf("failed to read solved response body: %w", err)
		}
		challengeResp.Body.Close()
		bodyStr = string(body)
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

	return newResp, instance, nil
}

func buildMediaURL(response *http.Response, path string) string {
	urlStr := fmt.Sprintf("%s://%s%s",
		string(response.Request.URL.Scheme),
		string(response.Request.URL.Host),
		path,
	)
	return cleanupRegex.ReplaceAllString(urlStr, "")
}

func extractRedlibCaption(body []byte) string {
	extract := func(regex string, body []byte) string {
		re := regexp.MustCompile(regex)
		if match := re.FindSubmatch(body); len(match) > 1 {
			return string(match[1])
		}
		return ""
	}

	postAuthor := extract(`(?s)<a class="post_author.*?" href=".*?">([^"]+)</a>`, body)
	postSubreddit := extract(`(?s)<a class="post_subreddit" href=".*?">([^"]+)</a>`, body)
	postTitle := extract(`(?s)<h1 class="post_title">(?:.*?</a>)?([^<]+)</h1>`, body)

	if postTitle != "" {
		postTitle = strings.Join(strings.Fields(postTitle), " ")
	}

	return fmt.Sprintf("<b>%s — %s</b>:\n%s",
		html.EscapeString(postAuthor),
		html.EscapeString(postSubreddit),
		html.EscapeString(postTitle))
}

func (h *Handler) processRedlibVideo(content []byte, response *http.Response) []gotgbot.InputMedia {
	if videoMatch := videoRegex.FindSubmatch(content); len(videoMatch) > 1 {
		playlistURL := buildMediaURL(response, string(playlistRegex.FindSubmatch(content)[1]))

		audioFile, err := downloadAudio(playlistURL)
		if err != nil {
			slog.Error("Failed to download audio",
				"Error", err.Error())
			return nil
		}

		thumbnail := h.downloadThumbnail(content, response)
		videoURL := buildMediaURL(response, string(videoMatch[1]))

		videoFile, err := downloader.FetchBytesFromURL(videoURL)
		if err != nil {
			slog.Error("Failed to download video",
				"Error", err.Error())
			return nil
		}

		videoFile, err = downloader.MergeAudioVideoBytes(videoFile, audioFile)
		if err != nil {
			slog.Error("Failed to merge audio and video",
				"Error", err.Error())
			return nil
		}

		filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Reddit_%s_%s", h.subreddit, h.postID))
		videoMedia := &gotgbot.InputMediaVideo{
			Media:             downloader.InputFileFromBytes(filename, videoFile),
			SupportsStreaming: true,
		}

		if thumbnail != nil {
			videoMedia.Thumbnail = downloader.InputFileFromBytes(filename, thumbnail)
		}
		return []gotgbot.InputMedia{videoMedia}
	}
	return nil
}

func downloadAudio(playlistURL string) ([]byte, error) {
	if playlistURL == "" {
		return nil, fmt.Errorf("empty playlist URL")
	}

	response, err := utils.Request(playlistURL, utils.RequestParams{Method: "GET"})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch audio playlist: %s", err)
	}
	defer response.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(response.Body, true)
	if err != nil || listType != m3u8.MASTER {
		return nil, fmt.Errorf("failed to decode audio playlist: %s", err)
	}

	audioVariant := getHighestQualityAudio(playlist.(*m3u8.MasterPlaylist))
	if audioVariant == nil {
		return nil, fmt.Errorf("failed to get highest quality audio variant")
	}

	audioURL := strings.ReplaceAll(
		fmt.Sprintf("%s://%s%s/%s",
			string(response.Request.URL.Scheme),
			string(response.Request.URL.Host),
			path.Dir(string(response.Request.URL.Path)),
			audioVariant.URI,
		), "m3u8", "aac")
	audioFile, err := downloader.FetchBytesFromURL(audioURL)
	if err != nil {
		return nil, err
	}

	return audioFile, nil
}

func getHighestQualityAudio(playlist *m3u8.MasterPlaylist) *m3u8.Alternative {
	var bestAudio *m3u8.Alternative
	for _, variant := range playlist.Variants {
		for _, audio := range variant.Alternatives {
			if bestAudio == nil || audio.GroupId > bestAudio.GroupId {
				bestAudio = audio
			}
		}
	}
	return bestAudio
}

func (h *Handler) downloadThumbnail(content []byte, response *http.Response) []byte {
	if thumbMatch := thumbRegex.FindSubmatch(content); len(thumbMatch) > 1 {
		thumbnailURL := buildMediaURL(response, string(thumbMatch[1]))

		thumbnail, err := downloader.FetchBytesFromURL(thumbnailURL)
		if err != nil {
			slog.Error("Failed to download thumbnail",
				"Thumbnail URL", thumbnailURL,
				"Error", err.Error())
			return nil
		}

		thumbnail, err = utils.ResizeThumbnail(thumbnail)
		if err != nil {
			slog.Error("Failed to resize thumbnail",
				"Thumbnail URL", thumbnailURL,
				"Error", err.Error())
		}

		return thumbnail
	}
	return nil
}

func (h *Handler) processRedlibImage(content []byte, response *http.Response) []gotgbot.InputMedia {
	if imageMatch := imageRegex.FindSubmatch(content); len(imageMatch) > 1 {
		imageURL := buildMediaURL(response, string(imageMatch[1]))

		file, err := downloader.FetchBytesFromURL(imageURL)
		if err != nil {
			slog.Error("Failed to download image",
				"Error", err.Error())
			return nil
		}

		filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Reddit_%s_%s", h.subreddit, h.postID))
		return []gotgbot.InputMedia{&gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromBytes(filename, file),
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

			fetchResult, err := downloader.FetchBytesFromURLWithClient(media, client)
			if err != nil {
				results <- mediaResult{index: index, err: err}
				return
			}

			bodyStr := string(fetchResult.Body[:min(500, len(fetchResult.Body))])
			if strings.Contains(bodyStr, "anubis") || strings.Contains(bodyStr, "Making sure") || strings.Contains(bodyStr, "block_page") {
				solved, solveErr := solveAnubisMediaForURL(media, client)
				if solveErr != nil {
					results <- mediaResult{index: index, err: solveErr}
					return
				}
				fetchResult = solved
			}

			file := fetchResult.Body
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
				"Error", result.err.Error())
			continue
		}
		mediaItems[result.index] = result.media
	}

	return mediaItems
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
		if err != nil {
			return nil
		}
		return &data.Data.Children[0].Data
	}
	defer response.Body.Close()

	var data RedditPost
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
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
