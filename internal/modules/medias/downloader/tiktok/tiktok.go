package tiktok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	postInfo, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return postInfo
	}

	if tikTokData := handler.getTikTokData(); tikTokData != nil {
		handler.username = *tikTokData.AwemeList[0].Author.Nickname

		if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
			return downloader.PostInfo{
				ID:      handler.postID,
				Medias:  handler.handleImages(tikTokData),
				Caption: getCaption(tikTokData),
			}
		}
		return downloader.PostInfo{
			ID:      handler.postID,
			Medias:  handler.handleVideo(tikTokData),
			Caption: getCaption(tikTokData),
		}
	}

	return downloader.PostInfo{}
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

func (h *Handler) getTikTokData() TikTokData {
	TikTokQueryParams["aweme_id"] = h.postID

	response, err := utils.Request("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestParams{
		Method:  "OPTIONS",
		Headers: downloader.GenericHeaders,
		Query:   TikTokQueryParams,
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

func (h *Handler) handleImages(tikTokData TikTokData) []models.InputMedia {
	type mediaResult struct {
		index int
		file  []byte
		err   error
	}

	mediaCount := len(tikTokData.AwemeList[0].ImagePostInfo.Images)
	mediaItems := make([]models.InputMedia, mediaCount)
	results := make(chan mediaResult, mediaCount)

	for i, media := range tikTokData.AwemeList[0].ImagePostInfo.Images {
		go func(index int, media Image) {
			file, err := downloader.FetchBytesFromURL(media.DisplayImage.URLList[1])
			if err != nil {
				slog.Error("Failed to download thumbnail",
					"Post Info", []string{h.username, h.postID},
					"Error", err.Error())
			}
			results <- mediaResult{index, file, err}
		}(i, media)
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
		if result.file != nil {
			mediaItems[result.index] = &models.InputMediaPhoto{
				Media: "attach://" + utils.SanitizeString(
					fmt.Sprintf("SmudgeLord-TikTok_%d_%s_%s", result.index, h.username, h.postID)),
				MediaAttachment: bytes.NewBuffer(result.file),
			}
		}
	}

	return mediaItems
}

func (h *Handler) handleVideo(tikTokData TikTokData) []models.InputMedia {
	file, err := downloader.FetchBytesFromURL(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(tikTokData.AwemeList[0].Video.Cover.URLList[0])
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
		Media: "attach://" + utils.SanitizeString(
			fmt.Sprintf("SmudgeLord-TikTok_%s_%s", h.username, h.postID)),
		Thumbnail: &models.InputFileUpload{
			Filename: "attach://" + utils.SanitizeString(
				fmt.Sprintf("SmudgeLord-TikTok_%s_%s", h.username, h.postID)),
			Data: bytes.NewBuffer(thumbnail),
		},
		Width:             tikTokData.AwemeList[0].Video.PlayAddr.Width,
		Height:            tikTokData.AwemeList[0].Video.PlayAddr.Height,
		Duration:          tikTokData.AwemeList[0].Video.Duration / 1000,
		SupportsStreaming: true,
		MediaAttachment:   bytes.NewBuffer(file),
	}}
}
