package reddit

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/valyala/fasthttp"
)

var redlibInstance = "redlib.catsarch.com"

func Handle(text string) ([]telego.InputMedia, []string) {
	postInfo := getPostInfo(text)
	if len(postInfo) < 2 {
		return nil, []string{}
	}
	medias, caption := processMedia(text)
	if medias == nil {
		return nil, []string{}
	}

	return medias, []string{caption, fmt.Sprintf("%s/%s", postInfo[1], postInfo[2])}
}

func getPostInfo(url string) []string {
	if matches := regexp.MustCompile(`(?:www.)?reddit.com/(?:user|r)/([^/]+)/comments/([^/]+)`).FindStringSubmatch(url); len(matches) > 2 {
		return matches
	}
	return []string{}
}

func buildMediaURL(request *fasthttp.Request, path string) string {
	return fmt.Sprintf("%s://%s%s",
		string(request.URI().Scheme()),
		string(request.URI().Host()),
		path,
	)
}

func processMedia(url string) ([]telego.InputMedia, string) {
	request, response, err := utils.Request(
		regexp.MustCompile(`(?:www.)?reddit.com`).ReplaceAllString(url, redlibInstance),
		utils.RequestParams{
			Method: "GET",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
			},
		})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response.Body() == nil {
		log.Println("Error fetching media content")
		return nil, ""
	}

	mediaContent := extractMediaContent(response.Body())
	if mediaContent == "" {
		return nil, ""
	}

	if videoMedia := processVideoMedia(mediaContent, request); videoMedia != nil {
		return videoMedia, extractCaption(response.Body())
	}

	if imageMedia := processImageMedia(mediaContent, request); imageMedia != nil {
		return imageMedia, extractCaption(response.Body())
	}

	return nil, ""
}

func extractMediaContent(body []byte) string {
	re := regexp.MustCompile(`(?s)<div class="post_media_content">(.*?)</div>`)
	if match := re.FindSubmatch(body); len(match) == 2 {
		return string(match[1])
	}
	return ""
}

func extractCaption(body []byte) string {
	extract := func(regex string, body []byte) string {
		re := regexp.MustCompile(regex)
		if match := re.FindSubmatch(body); len(match) > 1 {
			return string(match[1])
		}
		return ""
	}

	postAuthor := extract(`(?s)<a class="post_author.*?" href=".*?">([^"]+)</a>`, body)
	postSubreddit := extract(`(?s)<a class="post_subreddit" href=".*?">([^"]+)</a>`, body)
	postTitle := extract(`(?s)<h1 class="post_title">(?:.*?</a>)?\s*([^<]+)\s*</h1>`, body)

	caption := fmt.Sprintf("<b>%s — %s</b>: %s", postAuthor, postSubreddit, postTitle)
	return caption
}

func processVideoMedia(content string, request *fasthttp.Request) []telego.InputMedia {
	videoRegex := regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="video/mp4"`)
	if videoMatch := videoRegex.FindStringSubmatch(content); len(videoMatch) > 1 {
		thumbnail := downloadThumbnail(content, request)
		videoURL := buildMediaURL(request, videoMatch[1])

		file, err := downloader.Downloader(videoURL)
		if err != nil {
			log.Printf("Reddit — Error downloading video: %s", err)
			return nil
		}

		video := []telego.InputMedia{&telego.InputMediaVideo{
			Type:              telego.MediaTypeVideo,
			Media:             telego.InputFile{File: file},
			SupportsStreaming: true,
		}}

		if thumbnail != nil {
			video[0].(*telego.InputMediaVideo).Thumbnail = &telego.InputFile{File: thumbnail}
		}
		return video
	}
	return nil
}

func processImageMedia(content string, request *fasthttp.Request) []telego.InputMedia {
	imageRegex := regexp.MustCompile(`(?s)href="([^"]+).*?class="post_media_image"`)
	if imageMatch := imageRegex.FindStringSubmatch(content); len(imageMatch) > 1 {
		imageURL := buildMediaURL(request, imageMatch[1])

		file, err := downloader.Downloader(imageURL)
		if err != nil {
			log.Printf("Reddit — Error downloading image: %s", err)
			return nil
		}

		return []telego.InputMedia{&telego.InputMediaPhoto{
			Type:  telego.MediaTypePhoto,
			Media: telego.InputFile{File: file},
		}}
	}
	return nil
}

func downloadThumbnail(content string, request *fasthttp.Request) *os.File {
	thumbRegex := regexp.MustCompile(`(?s)class="post_media_video.*?poster="([^"]+)"`)
	if thumbMatch := thumbRegex.FindStringSubmatch(content); len(thumbMatch) > 1 {
		thumbnailURL := regexp.MustCompile(`(?s)#(\d+);`).ReplaceAllString(buildMediaURL(request, thumbMatch[1]), "")

		thumbnail, err := downloader.Downloader(thumbnailURL)
		if err != nil {
			log.Printf("Reddit — Error downloading thumbnail from %s: %s", thumbnailURL, err)
			return nil
		}

		err = utils.ResizeThumbnail(thumbnail)
		if err != nil {
			log.Printf("Reddit — Error resizing thumbnail: %s", err)
			return nil
		}

		return thumbnail
	}
	return nil
}
