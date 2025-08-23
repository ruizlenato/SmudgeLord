package tiktok

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Handler struct {
	postID string
}

func Handle(message string) ([]telegram.InputMedia, []string) {
	handler := &Handler{}
	if !handler.setPostID(message) {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, handler.postID}
	}

	tikTokData := getTikTokData(handler.postID)
	if tikTokData == nil {
		return nil, []string{}
	}

	caption := getCaption(tikTokData)

	if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
		return handler.handleImages(tikTokData), []string{caption, handler.postID}
	}
	return handler.handleVideo(tikTokData), []string{caption, handler.postID}
}

func (h *Handler) setPostID(url string) bool {
	postIDRegex := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`)
	if matches := postIDRegex.FindStringSubmatch(url); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	response, err := retryCaller.Request(url, utils.RequestParams{
		Method:    "GET",
		Redirects: 2,
	})

	if err != nil {
		return false
	}
	defer response.Body.Close()

	if matches := postIDRegex.FindStringSubmatch(response.Request.URL.String()); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}
	return false
}

func getTikTokData(postID string) TikTokData {
	response, err := utils.Request("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestParams{
		Method: "OPTIONS",
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		},
		Query: map[string]string{
			"iid":             "7318518857994389254",
			"device_id":       "7318517321748022790",
			"channel":         "googleplay",
			"version_code":    "300904",
			"device_platform": "android",
			"device_type":     "ASUS_Z01QD",
			"os_version":      "9",
			"aweme_id":        postID,
			"aid":             "1128",
		},
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	var tikTokData TikTokData
	err = json.NewDecoder(response.Body).Decode(&tikTokData)
	if err != nil {
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

func (h *Handler) handleImages(tikTokData TikTokData) []telegram.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
	}

	images := tikTokData.AwemeList[0].ImagePostInfo.Images
	mediaCount := len(images)
	if mediaCount > 10 {
		mediaCount = 10
		images = images[:10]
	}

	mediaItems := make([]telegram.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range images {
		go func(index int, media Image) {
			file, err := downloader.FetchBytesFromURL(media.DisplayImage.URLList[1])
			if err != nil {
				slog.Error(
					"Failed to download image",
					"Post ID", []string{h.postID},
					"Image URL", media.DisplayImage.URLList[1],
					"Error", err.Error(),
				)
			}
			results <- mediaResult{index, file}
		}(i, media)
	}

	for range mediaCount {
		result := <-results

		if result.file != nil {
			photo, err := helpers.UploadPhoto(helpers.UploadPhotoParams{
				File: result.file,
			})
			if err != nil {
				if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
					slog.Error(
						"Failed to upload image",
						"Post ID", []string{h.postID},
						"Image URL", images[result.index].DisplayImage.URLList[1],
						"Error", err.Error(),
					)
				}
				continue
			}
			mediaItems[result.index] = &photo
		}
	}

	return mediaItems
}

func (h *Handler) handleVideo(tikTokData TikTokData) []telegram.InputMedia {
	file, err := downloader.FetchBytesFromURL(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
	if err != nil {
		slog.Error(
			"Failed to download video",
			"Post ID", []string{h.postID},
			"Video URL", tikTokData.AwemeList[0].Video.PlayAddr.URLList[0],
			"Error", err.Error(),
		)
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(tikTokData.AwemeList[0].Video.Cover.URLList[0])
	if err != nil {
		slog.Error(
			"Failed to download thumbnail",
			"Post ID", []string{h.postID},
			"Thumbnail URL", tikTokData.AwemeList[0].Video.Cover.URLList[0],
			"Error", err.Error(),
		)
		return nil
	}

	thumbnail, err = utils.ResizeThumbnailFromBytes(thumbnail)
	if err != nil {
		slog.Error(
			"Failed to resize thumbnail",
			"Post ID", []string{h.postID},
			"Thumbnail URL", tikTokData.AwemeList[0].Video.Cover.URLList[0],
			"Error", err.Error(),
		)
	}

	video, err := helpers.UploadVideo(helpers.UploadVideoParams{
		File:              file,
		Thumb:             thumbnail,
		SupportsStreaming: true,
		Width:             int32(tikTokData.AwemeList[0].Video.PlayAddr.Width),
		Height:            int32(tikTokData.AwemeList[0].Video.PlayAddr.Height),
	})
	if err != nil {
		if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
			slog.Error(
				"Failed to upload video",
				"Post ID", []string{h.postID},
				"Video URL", tikTokData.AwemeList[0].Video.PlayAddr.URLList[0],
				"Thumbnail URL", tikTokData.AwemeList[0].Video.Cover.URLList[0],
				"Error", err.Error(),
			)
		}
		return nil
	}

	return []telegram.InputMedia{&video}
}
