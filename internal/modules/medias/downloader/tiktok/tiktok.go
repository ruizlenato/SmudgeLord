package tiktok

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
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

	if tikTokData := handler.getTikTokData(); tikTokData != nil {
		handler.username = *tikTokData.AwemeList[0].Author.Nickname
		caption := getCaption(tikTokData)

		if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
			return handler.downloadImages(tikTokData), []string{caption, handler.postID}
		}
		return handler.downloadVideo(tikTokData), []string{caption, handler.postID}
	}

	return nil, []string{}
}

func (h *Handler) setPostID(url string) bool {
	postIDRegex := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`)
	if matches := postIDRegex.FindStringSubmatch(url); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultFastHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	request, response, err := retryCaller.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		return false
	}

	if matches := postIDRegex.FindStringSubmatch(request.URI().String()); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}
	return false
}

func (h *Handler) getTikTokData() TikTokData {
	request, response, err := utils.Request("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestParams{
		Method:  "OPTIONS",
		Headers: downloader.GenericHeaders,
        Query: map[string]string{
			"iid":             "7318518857994389254",
			"device_id":       "7318517321748022790",
			"channel":         "googleplay",
			"version_code":    "300904",
			"device_platform": "android",
			"device_type":     "ASUS_Z01QD",
			"os_version":      "9",
			"aweme_id":        h.postID,
			"aid":             "1128",
		},
	})
	defer utils.ReleaseRequestResources(request, response)
	if err != nil || response.Body() == nil {
		return nil
	}

	var tikTokData TikTokData
	err = json.Unmarshal(response.Body(), &tikTokData)
	if err != nil {
		return nil
	}

	if tikTokData.AwemeList[0].AwemeID != h.postID {
		return nil
	}

	return tikTokData
}

func getCaption(tikTokData TikTokData) string {
	if len(tikTokData.AwemeList) == 0 {
		return ""
	}
	if tikTokData.AwemeList[0].Author.Nickname != nil && tikTokData.AwemeList[0].Desc != nil {
		return fmt.Sprintf("<b>%s</b>:\n%s", *tikTokData.AwemeList[0].Author.Nickname, *tikTokData.AwemeList[0].Desc)
	}
	return ""
}

func (h *Handler) downloadImages(tikTokData TikTokData) []telego.InputMedia {
	type mediaResult struct {
		index int
		file  *os.File
		err   error
	}

	mediaCount := len(tikTokData.AwemeList[0].ImagePostInfo.Images)
	mediaItems := make([]telego.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range tikTokData.AwemeList[0].ImagePostInfo.Images {
		go func(index int, media Image) {
			file, err := downloader.Downloader(media.DisplayImage.URLList[1], fmt.Sprintf("SmudgeLord-TikTok_%d_%s_%s", index, h.username, h.postID))
			if err != nil {
				slog.Error("Failed to download thumbnail",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error())
			}
			results <- mediaResult{index, file, err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.index,
				"Error", result.err)
			continue
		}
		if result.file != nil {
			mediaItems[result.index] = &telego.InputMediaPhoto{
				Type:  telego.MediaTypePhoto,
				Media: telego.InputFile{File: result.file},
			}
		}
	}

	return mediaItems
}

func (h *Handler) downloadVideo(tikTokData TikTokData) []telego.InputMedia {
	file, err := downloader.Downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0], fmt.Sprintf("SmudgeLord-TikTok_%s_%s", h.username, h.postID))
	if err != nil {
		slog.Error("Failed to download video",
			"PostID", tikTokData.AwemeList[0].AwemeID,
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.Downloader(tikTokData.AwemeList[0].Video.Cover.URLList[0])
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
	}

	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: file},
		Thumbnail:         &telego.InputFile{File: thumbnail},
		Width:             tikTokData.AwemeList[0].Video.PlayAddr.Width,
		Height:            tikTokData.AwemeList[0].Video.PlayAddr.Height,
		Duration:          tikTokData.AwemeList[0].Video.Duration / 1000,
		SupportsStreaming: true,
	}}
}
