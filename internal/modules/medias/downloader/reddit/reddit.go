package reddit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
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

var redlibInstance = "https://nyc1.lr.ggtyler.dev"

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

type Handler struct {
	subreddit string
	postID    string
}

func Handle(text string) ([]telego.InputMedia, []string) {
	handler := &Handler{}
	if !handler.getPostInfo(text) {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(fmt.Sprintf("%s/%s", handler.subreddit, handler.postID))
	if err == nil {
		return cachedMedias, []string{cachedCaption, fmt.Sprintf("%s/%s", handler.subreddit, handler.postID)}
	}

	medias, caption := handler.processMedia()
	if medias == nil {
		return nil, []string{}
	}

	return medias, []string{caption, fmt.Sprintf("%s/%s", handler.subreddit, handler.postID)}
}

func (h *Handler) getPostInfo(url string) bool {
	matches := postInfoRegex.FindStringSubmatch(url)
	if len(matches) < 3 {
		return false
	}

	h.subreddit = matches[1]
	h.postID = matches[2]
	return true
}

func (h *Handler) processMedia() ([]telego.InputMedia, string) {
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

func (h *Handler) getRedlibData() ([]telego.InputMedia, string) {
	request, response, err := utils.Request(fmt.Sprintf("%s/r/%s/comments/%s", redlibInstance, h.subreddit, h.postID),
		utils.RequestParams{
			Method: "GET",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
				"Cookie":     "use_hls=on",
			},
		})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil || response.Body() == nil {
		slog.Error("Failed to fetch media content",
			"Error", err.Error())
		return nil, ""
	}

	mediaContent := extractMediaContent(response.Body())
	if mediaContent == "" {
		return nil, ""
	}

	if videoMedia := h.processRedlibVideo(mediaContent, request); videoMedia != nil {
		return videoMedia, extractRedlibCaption(response.Body())
	}

	if imageMedia := h.processRedlibImage(mediaContent, request); imageMedia != nil {
		return imageMedia, extractRedlibCaption(response.Body())
	}

	return nil, ""
}

func buildMediaURL(request *fasthttp.Request, path string) string {
	return fmt.Sprintf("%s://%s%s",
		string(request.URI().Scheme()),
		string(request.URI().Host()),
		path,
	)
}

func extractMediaContent(body []byte) string {
	if match := mediaContentRegex.FindSubmatch(body); len(match) == 2 {
		return string(match[1])
	}
	return ""
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
	postTitle := extract(`(?s)<h1 class="post_title">(?:.*?</a>)?\s*([^<]+)\s*</h1>`, body)

	return fmt.Sprintf("<b>%s — %s</b>: %s", postAuthor, postSubreddit, postTitle)
}

func (h *Handler) processRedlibVideo(content string, request *fasthttp.Request) []telego.InputMedia {
	if videoMatch := videoRegex.FindStringSubmatch(content); len(videoMatch) > 1 {
		playlistURL := cleanupRegex.ReplaceAllString(buildMediaURL(request, playlistRegex.FindStringSubmatch(content)[1]), "")

		audioFile, err := downloadAudio(playlistURL)
		if err != nil {
			slog.Error("Failed to download audio",
				"Error", err.Error())
			return nil
		}

		thumbnail := h.downloadThumbnail(content, request)
		videoURL := buildMediaURL(request, videoMatch[1])

		videoFile, err := downloader.Downloader(videoURL)
		if err != nil {
			slog.Error("Failed to download video",
				"Error", err.Error())
			return nil
		}

		err = downloader.MergeAudioVideo(videoFile, audioFile)
		if err != nil {
			slog.Error("Failed to merge audio and video",
				"Error", err.Error())
			return nil
		}

		video := []telego.InputMedia{&telego.InputMediaVideo{
			Type:              telego.MediaTypeVideo,
			Media:             telego.InputFile{File: videoFile},
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
		return nil, fmt.Errorf("empty playlist URL")
	}

	request, response, err := utils.Request(playlistURL, utils.RequestParams{Method: "GET"})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch audio playlist: %s", err)
	}

	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(response.Body()), true)
	if err != nil || listType != m3u8.MASTER {
		return nil, fmt.Errorf("failed to decode audio playlist: %s", err)
	}

	audioVariant := getHighestQualityAudio(playlist.(*m3u8.MasterPlaylist))
	if audioVariant == nil {
		return nil, fmt.Errorf("failed to get highest quality audio variant")
	}

	audioURL := strings.ReplaceAll(
		fmt.Sprintf("%s://%s%s/%s",
			string(request.URI().Scheme()),
			string(request.URI().Host()),
			path.Dir(string(request.URI().Path())),
			audioVariant.URI,
		), "m3u8", "aac")
	audioFile, err := downloader.Downloader(audioURL)
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

func (h *Handler) processRedlibImage(content string, request *fasthttp.Request) []telego.InputMedia {
	if imageMatch := imageRegex.FindStringSubmatch(content); len(imageMatch) > 1 {
		imageURL := buildMediaURL(request, imageMatch[1])

		file, err := downloader.Downloader(imageURL)
		if err != nil {
			slog.Error("Failed to download image", "Error", err.Error())
			return nil
		}

		return []telego.InputMedia{&telego.InputMediaPhoto{
			Type:  telego.MediaTypePhoto,
			Media: telego.InputFile{File: file},
		}}
	}
	return nil
}

func (h *Handler) downloadThumbnail(content string, request *fasthttp.Request) *os.File {
	if thumbMatch := thumbRegex.FindStringSubmatch(content); len(thumbMatch) > 1 {
		thumbnailURL := cleanupRegex.ReplaceAllString(buildMediaURL(request, thumbMatch[1]), "")

		thumbnail, err := downloader.Downloader(thumbnailURL)
		if err != nil {
			slog.Error("Failed to download thumbnail", "Error", err.Error(), "Thumbnail URL", thumbnailURL)
			return nil
		}

		err = utils.ResizeThumbnail(thumbnail)
		if err != nil {
			slog.Error("Failed to resize thumbnail", "Error", err.Error())
		}

		return thumbnail
	}
	return nil
}

func (h *Handler) getAPIData() *Data {
	request, response, err := utils.Request(fmt.Sprintf("https://www.reddit.com/r/%s/comments/%s/.json?raw_json=1", h.subreddit, h.postID), utils.RequestParams{Method: "GET"})
	if err != nil || response.Body() == nil {
		request, response, err = utils.Request(fmt.Sprintf("https://api.reddit.com/api/info/?id=t3_%s", h.postID),
			utils.RequestParams{Method: "GET"})
		if err != nil || response.Body() == nil {
			return nil
		}
		defer utils.ReleaseRequestResources(request, response)

		var data KindData
		err = json.Unmarshal(response.Body(), &data)
		if err != nil {
			return nil
		}
		return &data.Data.Children[0].Data
	}

	defer utils.ReleaseRequestResources(request, response)

	var data RedditPost
	err = json.Unmarshal(response.Body(), &data)
	if err != nil {
		return nil
	}

	return &data[0].Data.Children[0].Data
}

func (h *Handler) processAPIMedia(data *Data) []telego.InputMedia {
	filename := fmt.Sprintf("SmudgeLord-Reddit_%s_%s", h.subreddit, h.postID)
	if data.IsVideo {
		video, err := downloader.Downloader(data.Media.RedditVideo.FallbackURL, filename)
		if err != nil {
			slog.Error("Failed to download video",
				"Error", err.Error())
			return nil
		}

		thumbnail, err := downloader.Downloader(data.Preview.Images[0].Source.URL)
		if err != nil {
			slog.Error("Failed to download thumbnail",
				"Error", err.Error())
			return nil
		}

		return []telego.InputMedia{&telego.InputMediaVideo{
			Type:              telego.MediaTypeVideo,
			Media:             telego.InputFile{File: video},
			Width:             data.Media.RedditVideo.Width,
			Height:            data.Media.RedditVideo.Height,
			Thumbnail:         &telego.InputFile{File: thumbnail},
			SupportsStreaming: true,
		}}
	}

	if data.MediaMetadata != nil {
		type mediaResult struct {
			index int
			media telego.InputMedia
			err   error
		}

		mediaCount := len(data.GalleryData.Items)
		mediaItems := make([]telego.InputMedia, mediaCount)
		results := make(chan mediaResult, mediaCount)

		for i, item := range data.GalleryData.Items {
			go func(index int, mediaID string) {
				media := (*data.MediaMetadata)[mediaID]
				file, err := downloader.Downloader(media.S.U)
				if err != nil {
					results <- mediaResult{index: index, err: err}
					return
				}

				var inputMedia telego.InputMedia
				if media.E == "Image" {
					inputMedia = &telego.InputMediaPhoto{
						Type:  telego.MediaTypePhoto,
						Media: telego.InputFile{File: file},
					}
				}
				results <- mediaResult{index: index, media: inputMedia, err: nil}
			}(i, item.MediaID)
		}

		for i := 0; i < mediaCount; i++ {
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

	if data.IsRedditMediaDomain && data.Domain == "i.redd.it" {
		image, err := downloader.Downloader(data.URL)
		if err != nil {
			slog.Error("Failed to download image",
				"Error", err.Error())
			return nil
		}

		return []telego.InputMedia{&telego.InputMediaPhoto{
			Type:  telego.MediaTypePhoto,
			Media: telego.InputFile{File: image},
		}}
	}

	return nil
}

func (h *Handler) processAPICaption(data *Data) string {
	return fmt.Sprintf("<b>%s — %s</b>: %s", data.SubredditNamePrefixed, data.Author, data.Title)
}
