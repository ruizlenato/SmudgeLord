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
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(message telego.Message) ([]telego.InputMedia, []string) {
	shortcode := getShortcode(message.Text)
	if shortcode == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(shortcode)
	if err == nil {
		return cachedMedias, []string{cachedCaption, shortcode}
	}

	threadsPost := getGQLData(getPostID(message.Text))
	if threadsPost == nil {
		return nil, []string{}
	}

	caption := getCaption(threadsPost)

	return processMedia(threadsPost.Data.Data.Edges[0].Node.ThreadItems[0].Post), []string{caption, shortcode}
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
		fmt.Println("No post_id")
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

func processMedia(post Post) []telego.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(post.CarouselMedia)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range post.CarouselMedia {
		go func(index int, twitterMedia CarouselMedia) {
			media, err := downloadMedia(twitterMedia)
			results <- mediaResult{index: index, media: media, err: err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			log.Print(result.err)
			continue
		}
		if result.media.File != nil {
			var mediaItem telego.InputMedia
			if post.CarouselMedia[result.index].VideoVersions != nil {
				mediaItem = &telego.InputMediaVideo{
					Type:              telego.MediaTypeVideo,
					Media:             telego.InputFile{File: result.media.File},
					Width:             post.CarouselMedia[result.index].OriginalWidth,
					Height:            post.CarouselMedia[result.index].OriginalHeight,
					SupportsStreaming: true,
				}
				if result.media.Thumbnail != nil {
					mediaItem.(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: result.media.Thumbnail}
				}
			} else {
				mediaItem = &telego.InputMediaPhoto{
					Type:  telego.MediaTypePhoto,
					Media: telego.InputFile{File: result.media.File},
				}
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func downloadMedia(threadsMedia CarouselMedia) (*InputMedia, error) {
	var media InputMedia
	var err error

	if threadsMedia.VideoVersions != nil {
		media.File, err = downloader.Downloader(threadsMedia.VideoVersions[0].URL)
		if err == nil {
			media.Thumbnail, _ = downloader.Downloader(threadsMedia.ImageVersions.Candidates[0].URL)
		}
	} else {
		media.File, err = downloader.Downloader(threadsMedia.ImageVersions.Candidates[0].URL)
	}
	if err != nil {
		return nil, err
	}

	return &media, nil
}
