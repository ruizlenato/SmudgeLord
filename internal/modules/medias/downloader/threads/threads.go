package threads

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot/models"

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

	return downloader.PostInfo{
		ID:      handler.postID,
		Medias:  handler.processMedia(graphQLData),
		Caption: getCaption(graphQLData),
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
	downloader.GenericHeaders["Content-Type"] = "application/x-www-form-urlencoded"
	downloader.GenericHeaders["X-Fb-Lsd"] = lsd
	downloader.GenericHeaders["X-Ig-App-Id"] = "238260118697367"
	downloader.GenericHeaders["Sec-Fetch-Mode"] = "cors"
	downloader.GenericHeaders["Sec-Fetch-Site"] = "same-origin"
	response, err := utils.Request("https://www.threads.com/api/graphql", utils.RequestParams{
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

func (h *Handler) processMedia(data ThreadsData) []models.InputMedia {
	post := data.Data.Data.Edges[0].Node.ThreadItems[0].Post

	switch {
	case post.CarouselMedia != nil:
		return h.handleCarousel(post)
	case len(post.VideoVersions) > 0:
		return h.handleVideo(post)
	case len(post.ImageVersions.Candidates) > 0:
		return h.handleImage(post)
	default:
		return nil
	}
}

func getCaption(threadsData ThreadsData) string {
	username := html.EscapeString(threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.User.Username)
	return fmt.Sprintf("<b><a href='https://www.threads.net/@%s'>%s</a>:</b>\n%s",
		username,
		username,
		html.EscapeString(threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.Caption.Text))
}

func (h *Handler) handleCarousel(post Post) []models.InputMedia {
	type mediaResult struct {
		index int
		media *downloader.InputMedia
		err   error
	}

	mediaCount := len(*post.CarouselMedia)
	mediaItems := make([]models.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, result := range *post.CarouselMedia {
		go func(index int, threadsMedia CarouselMedia) {
			var media downloader.InputMedia
			var err error
			if (*post.CarouselMedia)[index].VideoVersions == nil {
				media.File, err = downloader.FetchBytesFromURL(threadsMedia.ImageVersions.Candidates[0].URL)
			} else {
				media.File, err = downloader.FetchBytesFromURL(threadsMedia.VideoVersions[0].URL)
				if err == nil {
					media.Thumbnail, err = downloader.FetchBytesFromURL(threadsMedia.ImageVersions.Candidates[0].URL)
				}
			}
			results <- mediaResult{index: index, media: &media, err: err}
		}(i, result)
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
		if result.media.File != nil {
			var mediaItem models.InputMedia
			if (*post.CarouselMedia)[result.index].VideoVersions == nil {
				mediaItem = &models.InputMediaPhoto{
					Media: "attach://" + utils.SanitizeString(
						fmt.Sprintf("SmudgeLord-Threads_%d_%s_%s", result.index, h.username, h.postID)),
					MediaAttachment: bytes.NewBuffer(result.media.File),
				}
			} else {
				mediaItem = &models.InputMediaVideo{
					Media: "attach://" + utils.SanitizeString(
						fmt.Sprintf("SmudgeLord-Threads_%d_%s_%s", result.index, h.username, h.postID)),
					Width:             (*post.CarouselMedia)[result.index].OriginalWidth,
					Height:            (*post.CarouselMedia)[result.index].OriginalHeight,
					SupportsStreaming: true,
					MediaAttachment:   bytes.NewBuffer(result.media.File),
				}
				if result.media.Thumbnail != nil {
					thumbnail, err := utils.ResizeThumbnail(result.media.Thumbnail)
					if err != nil {
						slog.Error("Failed to resize thumbnail",
							"Post Info", []string{h.username, h.postID},
							"Error", err.Error())
					}
					mediaItem.(*models.InputMediaVideo).Thumbnail = &models.InputFileUpload{
						Filename: utils.SanitizeString(
							fmt.Sprintf("SmudgeLord-Threads_%d_%s_%s", result.index, h.username, h.postID)),
						Data: bytes.NewBuffer(thumbnail),
					}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func (h *Handler) handleVideo(post Post) []models.InputMedia {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Threads_%s_%s", h.username, h.postID))
	file, err := downloader.FetchBytesFromURL(post.VideoVersions[0].URL)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	thumbnail, err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}

	return []models.InputMedia{&models.InputMediaVideo{
		Media: "attach://" + filename,
		Thumbnail: &models.InputFileUpload{
			Filename: filename,
			Data:     bytes.NewBuffer(thumbnail),
		},
		Width:             post.OriginalWidth,
		Height:            post.OriginalHeight,
		SupportsStreaming: true,
		MediaAttachment:   bytes.NewBuffer(file),
	}}
}

func (h *Handler) handleImage(post Post) []models.InputMedia {
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Threads_%s_%s", h.username, h.postID))
	file, err := downloader.FetchBytesFromURL(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	return []models.InputMedia{&models.InputMediaPhoto{
		Media:           "attach://" + filename,
		MediaAttachment: bytes.NewBuffer(file),
	}}
}
