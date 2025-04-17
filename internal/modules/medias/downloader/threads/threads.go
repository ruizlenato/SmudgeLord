package threads

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	username string
	postID   string
}

func Handle(message *telegram.NewMessage) ([]telegram.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setPostID(message.Text()) {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, handler.postID}
	}

	graphQLData := getGQLData(handler.postID)
	if graphQLData == nil || graphQLData.Data.Data.Edges == nil {
		return nil, []string{}
	}

	if strings.HasPrefix(graphQLData.Data.Data.Edges[0].Node.ThreadItems[0].Post.TextPostAppInfo.LinkPreviewAttachment.DisplayURL, "instagram.com") {
		message.Message.Message = graphQLData.Data.Data.Edges[0].Node.ThreadItems[0].Post.TextPostAppInfo.LinkPreviewAttachment.URL
		medias, result := instagram.Handle(message)
		return medias, result
	}

	return handler.processMedia(graphQLData, message), []string{getCaption(graphQLData), handler.postID}
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

func getGQLData(postID string) ThreadsData {
	var threadsData ThreadsData

	lsd := utils.RandomString(10)
	downloader.GenericHeaders["Content-Type"] = "application/x-www-form-urlencoded"
	downloader.GenericHeaders["X-Fb-Lsd"] = lsd
	downloader.GenericHeaders["X-Ig-App-Id"] = "238260118697367"
	downloader.GenericHeaders["Sec-Fetch-Mode"] = "cors"
	downloader.GenericHeaders["Sec-Fetch-Site"] = "same-origin"
	response, err := utils.Request("https://www.threads.net/api/graphql", utils.RequestParams{
		Method:  "POST",
		Headers: downloader.GenericHeaders,
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
			"__relay_internal__pv__BarcelonaShouldShowFediverseM075Featuresrelayprovider":false}`, postID),
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
		slog.Error("Failed to unmarshal Threads GQLData", "Error", err.Error())
		return nil
	}

	return threadsData
}

func (h *Handler) processMedia(data ThreadsData, message *telegram.NewMessage) []telegram.InputMedia {
	post := data.Data.Data.Edges[0].Node.ThreadItems[0].Post

	switch {
	case post.CarouselMedia != nil:
		return h.handleCarousel(post, message)
	case len(post.VideoVersions) > 0:
		return h.handleVideo(post, message)
	case len(post.ImageVersions.Candidates) > 0:
		return h.handleImage(post, message)
	default:
		return nil
	}
}

func getCaption(threadsData ThreadsData) string {
	return fmt.Sprintf("<b>%s</b>:\n%s",
		threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.User.Username,
		threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.Caption.Text)
}

func (h *Handler) handleCarousel(post Post, message *telegram.NewMessage) []telegram.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
	}

	mediaCount := len(*post.CarouselMedia)
	mediaItems := make([]telegram.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, result := range *post.CarouselMedia {
		go func(index int, threadsMedia CarouselMedia) {
			var media InputMedia
			var err error
			if (*post.CarouselMedia)[index].VideoVersions == nil {
				media.File, err = downloader.FetchBytesFromURL(threadsMedia.ImageVersions.Candidates[0].URL)
				if err != nil {
					slog.Error("Failed to download image",
						"Post Info", []string{h.username, h.postID},
						"Image URL", threadsMedia.ImageVersions.Candidates[0].URL,
						"Error", err.Error())
				}
			} else {
				media.File, err = downloader.FetchBytesFromURL(threadsMedia.VideoVersions[0].URL)
				if err != nil {
					slog.Error("Failed to download video",
						"Post Info", []string{h.username, h.postID},
						"Video URL", threadsMedia.VideoVersions[0].URL,
						"Error", err.Error())
				}
				if err == nil {
					media.Thumbnail, err = downloader.FetchBytesFromURL(threadsMedia.ImageVersions.Candidates[0].URL)
					if err != nil {
						slog.Error("Failed to download thumbnail",
							"Post Info", []string{h.username, h.postID},
							"Thumbnail URL", threadsMedia.ImageVersions.Candidates[0].URL,
							"Error", err.Error())
					}
				}
			}
			results <- mediaResult{index: index, media: &media}
		}(i, result)
	}

	for range mediaCount {
		result := <-results

		if result.media.File != nil {
			var mediaItem telegram.InputMedia
			if (*post.CarouselMedia)[result.index].VideoVersions == nil {
				uploadedPhoto, _ := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
					File: result.media.File,
				})
				mediaItem = &uploadedPhoto
			} else {
				uploadedVideo, _ := helpers.UploadVideo(message, helpers.UploadVideoParams{
					File:              result.media.File,
					Thumb:             result.media.Thumbnail,
					SupportsStreaming: true,
					Width:             int32((*post.CarouselMedia)[result.index].OriginalWidth),
					Height:            int32((*post.CarouselMedia)[result.index].OriginalWidth),
				})
				mediaItem = &uploadedVideo
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func (h *Handler) handleVideo(post Post, message *telegram.NewMessage) []telegram.InputMedia {
	file, err := downloader.FetchBytesFromURL(post.VideoVersions[0].URL)
	if err != nil {
		slog.Error("Failed to download video",
			"PostID", h.postID,
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"PostID", h.postID,
			"Error", err.Error())
		return nil
	}

	uploadedVideo, _ := helpers.UploadVideo(message, helpers.UploadVideoParams{
		File:              file,
		Thumb:             thumbnail,
		SupportsStreaming: true,
		Width:             int32(post.OriginalWidth),
		Height:            int32(post.OriginalHeight),
	})
	return []telegram.InputMedia{&uploadedVideo}
}

func (h *Handler) handleImage(post Post, message *telegram.NewMessage) []telegram.InputMedia {
	file, err := downloader.FetchBytesFromURL(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	uploadedPhoto, _ := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
		File: file,
	})

	return []telegram.InputMedia{&uploadedPhoto}
}
