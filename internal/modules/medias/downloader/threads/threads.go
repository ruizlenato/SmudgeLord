package threads

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(handler.postID); err == nil {
		return postInfo
	}

	graphQLData := handler.getThreadsData()
	if graphQLData == nil || graphQLData.Data.Data.Edges == nil {
		return downloader.PostInfo{}
	}

	if strings.HasPrefix(graphQLData.Data.Data.Edges[0].Node.ThreadItems[0].Post.TextPostAppInfo.LinkPreviewAttachment.DisplayURL, "instagram.com") {
		return instagram.Handle(graphQLData.Data.Data.Edges[0].Node.ThreadItems[0].Post.TextPostAppInfo.LinkPreviewAttachment.URL)
	}

	medias, cleanup := handler.processMedia(graphQLData)

	return downloader.PostInfo{
		ID:      handler.postID,
		Medias:  medias,
		Caption: getCaption(graphQLData),
		Cleanup: cleanup,
	}
}

func (h *Handler) setPostID(url string) bool {
	response, err := utils.Request(url, utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent":     downloader.GenericHeaders["User-Agent"],
			"Sec-Fetch-Mode": "navigate",
		},
	})

	if err != nil {
		return false
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Error("Failed to read response body",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return false
	}

	idLocation := strings.Index(string(body), "post_id")
	if idLocation == -1 {
		return false
	}

	start := idLocation + 10
	end := strings.Index(string(body)[start:], "\"")
	if end == -1 {
		return false
	}
	h.postID = string(body)[start : start+end]
	return true
}

func (h *Handler) getThreadsData() ThreadsData {
	var threadsData ThreadsData

	lsd := utils.RandomString(10)
	headers := downloader.CloneHeaders(downloader.GenericHeaders)
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	headers["X-Fb-Lsd"] = lsd
	headers["X-Ig-App-Id"] = "238260118697367"
	headers["Sec-Fetch-Mode"] = "cors"
	headers["Sec-Fetch-Site"] = "same-origin"
	response, err := utils.Request("https://www.threads.com/api/graphql", utils.RequestParams{
		Method:  "POST",
		Headers: headers,
		BodyString: []string{
			fmt.Sprintf(`variables={
			"first":1,
			"postID":"%v",
			"__relay_internal__pv__BarcelonaIsLoggedInrelayprovider":false,
			"__relay_internal__pv__BarcelonaIsThreadContextHeaderEnabledrelayprovider":false,
			"__relay_internal__pv__BarcelonaIsThreadContextHeaderFollowButtonEnabledrelayprovider":false,
			"__relay_internal__pv__BarcelonaUseCometVideoPlaybackEnginerelayprovider":false,
			"__relay_internal__pv__BarcelonaOptionalCookiesEnabledrelayprovider":false,
			"__relay_internal__pv__BarcelonaIsViewCountEnabledrelayprovider":false,
			"__relay_internal__pv__BarcelonaShouldShowFediverseM075Featuresrelayprovider":false}`, h.postID),
			`doc_id=7448594591874178`,
			`lsd=` + lsd,
		},
	})

	if err != nil {
		return nil
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&threadsData)
	if err != nil {
		slog.Error("Failed to unmarshal Threads GQLData",
			"Error", err.Error())
		return nil
	}

	return threadsData
}

func (h *Handler) processMedia(data ThreadsData) ([]gotgbot.InputMedia, func()) {
	post := data.Data.Data.Edges[0].Node.ThreadItems[0].Post

	switch {
	case post.CarouselMedia != nil:
		return h.handleCarousel(post)
	case len(post.VideoVersions) > 0:
		return h.handleVideo(post)
	case len(post.ImageVersions.Candidates) > 0:
		return h.handleImage(post)
	default:
		return nil, nil
	}
}

func getCaption(threadsData ThreadsData) string {
	username := html.EscapeString(threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.User.Username)
	return fmt.Sprintf("<b><a href='https://www.threads.net/@%s'>%s</a>:</b>\n%s",
		username,
		username,
		html.EscapeString(threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.Caption.Text))
}

func (h *Handler) handleCarousel(post Post) ([]gotgbot.InputMedia, func()) {
	type mediaResult struct {
		index   int
		media   gotgbot.InputMedia
		cleanup func()
		err     error
	}

	mediaCount := len(*post.CarouselMedia)
	mediaItems := make([]gotgbot.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, result := range *post.CarouselMedia {
		go func(index int, threadsMedia CarouselMedia) {
			filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Threads_%d_%s_%s", index, h.username, h.postID))
			if threadsMedia.VideoVersions == nil {
				stream, cleanup, err := downloader.FetchStreamFromURL(threadsMedia.ImageVersions.Candidates[0].URL)
				if err != nil {
					results <- mediaResult{index: index, err: err}
					return
				}
				results <- mediaResult{
					index:   index,
					media:   &gotgbot.InputMediaPhoto{Media: downloader.InputFileFromReader(filename, stream)},
					cleanup: cleanup,
				}
				return
			}

			stream, cleanup, err := downloader.FetchStreamFromURL(threadsMedia.VideoVersions[0].URL)
			if err != nil {
				results <- mediaResult{index: index, err: err}
				return
			}

			thumbnail, thumbErr := downloader.FetchBytesFromURL(threadsMedia.ImageVersions.Candidates[0].URL)
			if thumbErr != nil {
				if cleanup != nil {
					cleanup()
				}
				results <- mediaResult{index: index, err: thumbErr}
				return
			}

			videoMedia := &gotgbot.InputMediaVideo{
				Media:             downloader.InputFileFromReader(filename, stream),
				Width:             int64((*post.CarouselMedia)[index].OriginalWidth),
				Height:            int64((*post.CarouselMedia)[index].OriginalHeight),
				SupportsStreaming: true,
			}
			if thumbnail != nil {
				if thumbnailBytes, resizeErr := utils.ResizeThumbnail(thumbnail); resizeErr != nil {
					slog.Error("Failed to resize thumbnail",
						"Post Info", []string{h.username, h.postID},
						"Error", resizeErr.Error())
				} else {
					videoMedia.Thumbnail = downloader.InputFileFromBytes(filename, thumbnailBytes)
				}
			}

			results <- mediaResult{index: index, media: videoMedia, cleanup: cleanup}
		}(i, result)
	}

	var cleanups []func()
	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	for range mediaCount {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index,
				"Error", result.err.Error())
			continue
		}
		addCleanup(result.cleanup)
		if result.media != nil {
			mediaItems[result.index] = result.media
		}
	}

	return mediaItems, downloader.CombineCleanups(cleanups...)
}

func (h *Handler) handleVideo(post Post) ([]gotgbot.InputMedia, func()) {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Threads_%s_%s", h.username, h.postID))
	file, cleanup, err := downloader.FetchStreamFromURL(post.VideoVersions[0].URL)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	thumbnail, err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}

	return []gotgbot.InputMedia{&gotgbot.InputMediaVideo{
		Media:             downloader.InputFileFromReader(filename, file),
		Thumbnail:         downloader.InputFileFromBytes(filename, thumbnail),
		Width:             int64(post.OriginalWidth),
		Height:            int64(post.OriginalHeight),
		SupportsStreaming: true,
	}}, cleanup
}

func (h *Handler) handleImage(post Post) ([]gotgbot.InputMedia, func()) {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Threads_%s_%s", h.username, h.postID))
	file, cleanup, err := downloader.FetchStreamFromURL(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	return []gotgbot.InputMedia{&gotgbot.InputMediaPhoto{
		Media: downloader.InputFileFromReader(filename, file),
	}}, cleanup
}
