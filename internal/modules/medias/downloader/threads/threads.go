package threads

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	username string
	postID   string
}

func Handle(text string) ([]telego.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setPostID(text) {
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
		medias, result := instagram.Handle(graphQLData.Data.Data.Edges[0].Node.ThreadItems[0].Post.TextPostAppInfo.LinkPreviewAttachment.URL)
		return medias, result
	}

	return handler.processMedia(graphQLData), []string{getCaption(graphQLData), handler.postID}
}

func (h *Handler) setPostID(url string) bool {
	request, response, err := utils.Request(url, utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent":     downloader.GenericHeaders["User-Agent"],
			"Sec-Fetch-Mode": "navigate",
		},
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return false
	}

	idLocation := strings.Index(string(response.Body()), "post_id")
	if idLocation == -1 {
		return false
	}

	start := idLocation + 10
	end := strings.Index(string(response.Body())[start:], "\"")
	if end == -1 {
		return false
	}
	h.postID = string(response.Body())[start : start+end]
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
	request, response, err := utils.Request("https://www.threads.net/api/graphql", utils.RequestParams{
		Method: "POST",
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
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return nil
	}

	err = json.Unmarshal(response.Body(), &threadsData)
	if err != nil {
		slog.Error("Failed to unmarshal Threads GQLData", "Error", err.Error())
		return nil
	}

	return threadsData
}

func (h *Handler) processMedia(data ThreadsData) []telego.InputMedia {
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
	return fmt.Sprintf("<b>%s</b>:\n%s",
		threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.User.Username,
		threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.Caption.Text)
}

type InputMedia struct {
	File      *os.File
	Thumbnail *os.File
}

func (h *Handler) handleCarousel(post Post) []telego.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(*post.CarouselMedia)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, result := range *post.CarouselMedia {
		go func(index int, threadsMedia CarouselMedia) {
			var media InputMedia
			var err error
			if (*post.CarouselMedia)[index].VideoVersions == nil {
				media.File, err = downloader.Downloader(threadsMedia.ImageVersions.Candidates[0].URL, fmt.Sprintf("SmudgeLord-Threads_%d_%s_%s", index, h.username, h.postID))
			} else {
				media.File, err = downloader.Downloader(threadsMedia.VideoVersions[0].URL, fmt.Sprintf("SmudgeLord-Threads_%d_%s_%s", index, h.username, h.postID))
				if err == nil {
					media.Thumbnail, err = downloader.Downloader(threadsMedia.ImageVersions.Candidates[0].URL)
				}
			}
			results <- mediaResult{index: index, media: &media, err: err}
		}(i, result)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"PostID", h.postID,
				"Media Count", result.index,
				"Error", result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if (*post.CarouselMedia)[result.index].VideoVersions == nil {
				mediaItem = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: result.media.File},
				}
			} else {
				mediaItem = &telego.InputMediaVideo{
					Type:              telego.MediaTypeVideo,
					Media:             telego.InputFile{File: result.media.File},
					Width:             (*post.CarouselMedia)[result.index].OriginalWidth,
					Height:            (*post.CarouselMedia)[result.index].OriginalHeight,
					SupportsStreaming: true,
				}
				if result.media.Thumbnail != nil {
					mediaItem.(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: result.media.Thumbnail}
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func (h *Handler) handleVideo(post Post) []telego.InputMedia {
	file, err := downloader.Downloader(post.VideoVersions[0].URL)
	if err != nil {
		slog.Error("Failed to download video",
			"PostID", h.postID,
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.Downloader(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"PostID", h.postID,
			"Error", err.Error())
		return nil
	}

	err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"PostID", h.postID,
			"Error", err.Error())
	}

	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: file},
		Thumbnail:         &telego.InputFile{File: thumbnail},
		Width:             post.OriginalWidth,
		Height:            post.OriginalHeight,
		SupportsStreaming: true,
	}}
}

func (h *Handler) handleImage(post Post) []telego.InputMedia {
	file, err := downloader.Downloader(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		slog.Error("Failed to download image",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	return []telego.InputMedia{&telego.InputMediaPhoto{
		Type:  telego.MediaTypePhoto,
		Media: telego.InputFile{File: file},
	}}
}
