package threads

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(message *telegram.NewMessage) ([]telegram.InputMedia, []string) {
	var medias []telegram.InputMedia
	shortcode := getShortcode(message.Text())
	if shortcode == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(shortcode)
	if err == nil {
		return cachedMedias, []string{cachedCaption, shortcode}
	}

	postID := getPostID(message.Text())
	if postID == "" {
		return nil, []string{}
	}

	graphQLData := getGQLData(postID)
	if graphQLData == nil || graphQLData.Data.Data.Edges == nil || len(graphQLData.Data.Data.Edges) == 0 {
		return nil, []string{}
	}

	edge := graphQLData.Data.Data.Edges[0]
	if edge.Node.ThreadItems == nil || len(edge.Node.ThreadItems) == 0 {
		return nil, []string{}
	}

	threadsPost := edge.Node.ThreadItems[0].Post
	switch {
	case threadsPost.CarouselMedia != nil:
		medias = handleCarousel(threadsPost, message)
	case len(threadsPost.VideoVersions) > 0:
		medias = handleVideo(threadsPost, message)
	case len(threadsPost.ImageVersions.Candidates) > 0:
		medias = handleImage(threadsPost, message)
	}

	caption := getCaption(graphQLData)

	return medias, []string{caption, ""}
}

func getShortcode(url string) string {
	re := regexp.MustCompile(`(?:post)/([A-Za-z0-9_-]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func getPostID(message string) string {
	response, err := utils.Request(message, utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent":     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
			"Sec-Fetch-Mode": "navigate",
		},
	})
	defer response.Body.Close()

	if err != nil {
		return ""
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Print("Threads — Error reading body")
		return ""
	}

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
	lsd := utils.RandomString(10)

	response, err := utils.Request("https://www.threads.net/api/graphql", utils.RequestParams{
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
	})
	defer response.Body.Close()

	if err != nil {
		return nil
	}

	var threadsData ThreadsData
	err = json.NewDecoder(response.Body).Decode(&threadsData)
	if err != nil {
		return nil
	}

	return threadsData
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

func handleCarousel(post Post, message *telegram.NewMessage) []telegram.InputMedia {
	type mediaResult struct {
		index int
		media *InputMedia
		err   error
	}

	mediaCount := len(*post.CarouselMedia)
	mediaItems := make([]telegram.InputMedia, mediaCount)
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
			var mediaItem telegram.InputMedia
			if (*post.CarouselMedia)[result.index].VideoVersions == nil {
				photo, _ := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
					File: result.media.File.Name(),
				})
				mediaItem = &photo
			} else {
				uploadedVideo, _ := helpers.UploadDocument(message, helpers.UploadDocumentParams{
					File:  result.media.File.Name(),
					Thumb: result.media.Thumbnail.Name(),
					Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
						SupportsStreaming: true,
						W:                 int32((*post.CarouselMedia)[result.index].OriginalWidth),
						H:                 int32((*post.CarouselMedia)[result.index].OriginalHeight),
					}},
				})
				mediaItem = &uploadedVideo
			}
			mediaItems[result.index] = mediaItem
		}
	}

	return mediaItems
}

func handleVideo(post Post, message *telegram.NewMessage) []telegram.InputMedia {
	file, err := downloader.Downloader(post.VideoVersions[0].URL)
	if err != nil {
		log.Print("Threads — Error downloading video: ", err)
		return nil
	}

	thumbnail, err := downloader.Downloader(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		log.Print("Threads — Error downloading thumbnail: ", err)
		return nil
	}
	uploadedVideo, _ := helpers.UploadDocument(message, helpers.UploadDocumentParams{
		File:  file.Name(),
		Thumb: thumbnail.Name(),
		Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
			SupportsStreaming: true,
			W:                 int32(post.OriginalWidth),
			H:                 int32(post.OriginalHeight),
		}},
	})

	return []telegram.InputMedia{&uploadedVideo}
}

func handleImage(post Post, message *telegram.NewMessage) []telegram.InputMedia {
	file, err := downloader.Downloader(post.ImageVersions.Candidates[0].URL)
	if err != nil {
		log.Print("Threads: Error downloading image:", err)
		return nil
	}
	uploadedImage, _ := helpers.UploadPhoto(message, helpers.UploadPhotoParams{
		File: file.Name(),
	})

	return []telegram.InputMedia{&uploadedImage}
}
