package reddit

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/grafov/m3u8"
	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/valyala/fasthttp"
)

var redlibInstance = "nyc1.lr.ggtyler.dev"

// Regex patterns
var (
	postInfoRegex     = regexp.MustCompile(`(?:www.)?reddit.com/(?:user|r)/([^/]+)/comments/([^/]+)`)
	redditURLRegex    = regexp.MustCompile(`(?:www.)?reddit.com`)
	mediaContentRegex = regexp.MustCompile(`(?s)<div class="post_media_content">(.*?)</div>`)
	videoRegex        = regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="video/mp4"`)
	playlistRegex     = regexp.MustCompile(`(?s)class="post_media_video.*?<source\s+src="([^"]+)"\s+type="application/vnd.apple.mpegurl"`)
	imageRegex        = regexp.MustCompile(`(?s)href="([^"]+).*?class="post_media_image"`)
	thumbRegex        = regexp.MustCompile(`(?s)class="post_media_video.*?poster="([^"]+)"`)
	cleanupRegex      = regexp.MustCompile(`(?s)(?:#\d+|amp);`)
)

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
	if matches := postInfoRegex.FindStringSubmatch(url); len(matches) > 2 {
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
		redditURLRegex.ReplaceAllString(url, redlibInstance),
		utils.RequestParams{
			Method: "GET",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
				"Cookie":     "use_hls=on",
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
	if match := mediaContentRegex.FindSubmatch(body); len(match) == 2 {
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

	return fmt.Sprintf("<b>%s — %s</b>: %s", postAuthor, postSubreddit, postTitle)
}

func processVideoMedia(content string, request *fasthttp.Request) []telego.InputMedia {
	if videoMatch := videoRegex.FindStringSubmatch(content); len(videoMatch) > 1 {
		playlistURL := cleanupRegex.ReplaceAllString(buildMediaURL(request, playlistRegex.FindStringSubmatch(content)[1]), "")

		audioFile, err := downloadAudio(playlistURL)
		if err != nil {
			log.Print("Reddit — Error downloading audio: ", err.Error())
			return nil
		}

		thumbnail := downloadThumbnail(content, request)
		videoURL := buildMediaURL(request, videoMatch[1])

		videoFile, err := downloader.Downloader(videoURL)
		if err != nil {
			log.Print("Reddit — Error downloading video: ", err.Error())
			return nil
		}

		file := downloader.MergeAudioVideo(audioFile, videoFile)

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

func downloadAudio(playlistURL string) (*os.File, error) {
	if playlistURL == "" {
		return nil, fmt.Errorf("Empty playlist URL")
	}

	request, response, err := utils.Request(playlistURL, utils.RequestParams{Method: "GET"})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return nil, fmt.Errorf("Failed to fetch audio playlist: %s", err)
	}

	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(response.Body()), true)
	if err != nil || listType != m3u8.MASTER {
		return nil, fmt.Errorf("Failed to decode audio playlist: %s", err)
	}

	audioVariant := getHighestQualityAudio(playlist.(*m3u8.MasterPlaylist))
	if audioVariant == nil {
		return nil, fmt.Errorf("Failed to get highest quality audio variant")
	}

	audioURL := buildAudioURL(request, audioVariant.URI)
	audioFile, err := downloader.Downloader(audioURL)
	if err != nil {
		return nil, err
	}

	return audioFile, nil
}

func buildAudioURL(request *fasthttp.Request, audioPath string) string {
	return strings.ReplaceAll(
		fmt.Sprintf("%s://%s%s/%s",
			string(request.URI().Scheme()),
			string(request.URI().Host()),
			path.Dir(string(request.URI().Path())),
			audioPath,
		),
		"m3u8",
		"aac",
	)
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

func processImageMedia(content string, request *fasthttp.Request) []telego.InputMedia {
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
	if thumbMatch := thumbRegex.FindStringSubmatch(content); len(thumbMatch) > 1 {
		thumbnailURL := cleanupRegex.ReplaceAllString(buildMediaURL(request, thumbMatch[1]), "")

		thumbnail, err := downloader.Downloader(thumbnailURL)
		if err != nil {
			log.Printf("Reddit — Error downloading thumbnail from %s: %s", thumbnailURL, err)
			return nil
		}

		err = utils.ResizeThumbnail(thumbnail)
		if err != nil {
			log.Printf("Reddit — Error resizing thumbnail: %s", err)
		}

		return thumbnail
	}
	return nil
}
