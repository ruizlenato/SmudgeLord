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

func Handle(text string) ([]telego.InputMedia, []string) {
	postID := getPostID(text)
	if postID == "" {
		return nil, []string{}
	}

	cachedMedias, cachedCaption, err := downloader.GetMediaCache(postID)
	if err == nil {
		return cachedMedias, []string{cachedCaption, postID}
	}

	tikTokData := getTikTokData(postID)
	if tikTokData == nil {
		return nil, []string{}
	}

	caption := getCaption(tikTokData)

	if slices.Contains([]int{2, 68, 150}, tikTokData.AwemeList[0].AwemeType) {
		return downloadImages(tikTokData), []string{caption, postID}
	}
	return downloadVideo(tikTokData), []string{caption, postID}
}

func getPostID(url string) (postID string) {
	postIDRegex := regexp.MustCompile(`/(?:video|photo|v)/(\d+)`)

	if matches := postIDRegex.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
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
		return postID
	}

	if matches := postIDRegex.FindStringSubmatch(request.URI().String()); len(matches) > 1 {
		return matches[1]
	}

	return postID
}

func getTikTokData(postID string) TikTokData {
	request, response, err := utils.Request("https://api16-normal-c-useast1a.tiktokv.com/aweme/v1/feed/", utils.RequestParams{
		Method: "OPTIONS",
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
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
	defer utils.ReleaseRequestResources(request, response)
	if err != nil || response.Body() == nil {
		return nil
	}

	var tikTokData TikTokData
	err = json.Unmarshal(response.Body(), &tikTokData)
	if err != nil {
		return nil
	}

	if tikTokData.AwemeList[0].AwemeID != postID {
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

func downloadImages(tikTokData TikTokData) []telego.InputMedia {
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
			file, err := downloader.Downloader(media.DisplayImage.URLList[1])
			if err != nil {
				slog.Error("Couldn't download thumbnail", "PostID", tikTokData.AwemeList[0].AwemeID, "Error", err.Error())
			}
			results <- mediaResult{index, file, err}
		}(i, media)
	}

	for i := 0; i < mediaCount; i++ {
		result := <-results
		if result.err != nil {
			slog.Error("Couldn't download media in carousel", "Media Count", result.index, "Error", result.err)
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

func downloadVideo(tikTokData TikTokData) []telego.InputMedia {
	file, err := downloader.Downloader(tikTokData.AwemeList[0].Video.PlayAddr.URLList[0])
	if err != nil {
		slog.Error("Couldn't download video", "PostID", tikTokData.AwemeList[0].AwemeID, "Error", err.Error())
		return nil
	}

	thumbnail, err := downloader.Downloader(tikTokData.AwemeList[0].Video.Cover.URLList[0])
	if err != nil {
		slog.Error("Couldn't download thumbnail", "PostID", tikTokData.AwemeList[0].AwemeID, "Error", err.Error())
		return nil
	}

	err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Couldn't resize thumbnail", "PostID", tikTokData.AwemeList[0].AwemeID, "Error", err.Error())
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
