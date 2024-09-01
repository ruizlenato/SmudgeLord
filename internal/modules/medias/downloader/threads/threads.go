package threads

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) ([]telego.InputMedia, []string) {
	var medias []telego.InputMedia
	shortcode := getShortcode(text)
	if shortcode == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(shortcode)
	if err == nil {
		return cachedMedias, []string{cachedCaption, shortcode}
	}

	graphQLData := getGQLData(getPostID(text))
	if graphQLData == nil || graphQLData.Data.Data.Edges == nil {
		return nil, []string{}
	}

	threadsPost := graphQLData.Data.Data.Edges[0].Node.ThreadItems[0].Post
	if threadsPost.CarouselMedia != nil {
		medias = handleCarousel(threadsPost)
	} else if len(threadsPost.VideoVersions) > 0 {
		medias = handleVideo(threadsPost)
	} else if len(threadsPost.ImageVersions.Candidates) > 0 {
		medias = handleImage(threadsPost)
	}

	if strings.HasPrefix(threadsPost.TextPostAppInfo.LinkPreviewAttachment.DisplayURL, "instagram.com") {
		medias, result := instagram.Handle(threadsPost.TextPostAppInfo.LinkPreviewAttachment.URL)
		return medias, result
	}

	caption := getCaption(graphQLData)

	return medias, []string{caption, shortcode}
}

func getShortcode(url string) (postID string) {
	if matches := regexp.MustCompile(`(?:post)/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 2 {
		return matches[1]
	}
	return postID
}

func getPostID(message string) string {
	body := utils.Request(message, utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent":     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
			"Sec-Fetch-Mode": "navigate",
		},
	}).Body()

	idLocation := strings.Index(string(body), "post_id")
	if idLocation == -1 {
		return ""
	}

	start := idLocation + 10
	end := strings.Index(string(body)[start:], "\"")
	if end == -1 {
		return ""
	}

	return string(body)[start : start+end]
}

func getGQLData(postID string) ThreadsData {
	var threadsData ThreadsData

	lsd := utils.RandomString(10)

	body := utils.Request("https://www.threads.net/api/graphql", utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:     `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36`,
			`Content-Type`:   `application/x-www-form-urlencoded`,
			`X-Fb-Lsd`:       lsd,
			`X-Ig-App-Id`:    `238260118697367`,
			`Sec-Fetch-Mode`: `cors`,
			`Sec-Fetch-Site`: `same-origin`,
		},
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
	}).Body()

	err := json.Unmarshal(body, &threadsData)
	if err != nil {
		log.Print("threadsData: Error unmarshalling GQLData: ", err)
		return nil
	}

	return threadsData
}

func getCaption(threadsData ThreadsData) string {
	var caption string

	caption = fmt.Sprintf("<b>%s</b>:\n%s",
		threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.User.Username,
		threadsData.Data.Data.Edges[0].Node.ThreadItems[0].Post.Caption.Text)

	return caption
}

type InputMedia struct {
	File      *os.File
	Thumbnail *os.File
}

func handleCarousel(post Post) []telego.InputMedia {
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
				media.File, err = downloader.Downloader(threadsMedia.ImageVersions.Candidates[0].URL)
			} else {
				media.File, err = downloader.Downloader(threadsMedia.VideoVersions[0].URL)
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
			log.Print(result.err)
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

func handleVideo(post Post) []telego.InputMedia {
	file, err := downloader.Downloader(post.VideoVersions[0].URL)
	if err != nil {
		log.Print("Threads: Error downloading video: ", err)
		return nil
	}

	thumbnail, err := downloader.Downloader(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		log.Print("Threads: Error downloading thumbnail: ", err)
		return nil
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

func handleImage(post Post) []telego.InputMedia {
	file, err := downloader.Downloader(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		log.Print("Threads: Error downloading image:", err)
		return nil
	}

	return []telego.InputMedia{&telego.InputMediaPhoto{
		Type:  telego.MediaTypePhoto,
		Media: telego.InputFile{File: file},
	}}
}
