package pinterest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(handler.postID); err == nil {
		return postInfo
	}

	medias, caption, cleanup := handler.processMedia()
	if medias == nil {
		return downloader.PostInfo{ID: handler.postID, NoMedia: true, Cleanup: cleanup}
	}

	return downloader.PostInfo{
		ID:      handler.postID,
		Medias:  medias,
		Caption: caption,
		Cleanup: cleanup,
	}
}

func (h *Handler) setPostID(url string) bool {
	if matches := pinURLRegex.FindStringSubmatch(url); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	if matches := shortLinkRegex.FindStringSubmatch(url); len(matches) > 1 {
		resolvedID := h.resolveShortLink(matches[1])
		if resolvedID != "" {
			h.postID = resolvedID
			return true
		}
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
		Redirects: 4,
	})
	if err != nil {
		return false
	}
	defer response.Body.Close()

	finalURL := response.Request.URL.String()
	if matches := pinURLRegex.FindStringSubmatch(finalURL); len(matches) > 1 {
		h.postID = matches[1]
		return true
	}

	return false
}

func (h *Handler) resolveShortLink(shortID string) string {
	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   1 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	response, err := retryCaller.Request(
		fmt.Sprintf("https://api.pinterest.com/url_shortener/%s/redirect/", shortID),
		utils.RequestParams{
			Method:    "GET",
			Redirects: 4,
		},
	)
	if err != nil || response == nil {
		return ""
	}
	defer response.Body.Close()

	finalURL := response.Request.URL.String()
	if matches := pinURLRegex.FindStringSubmatch(finalURL); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (h *Handler) processMedia() ([]gotgbot.InputMedia, string, func()) {
	htmlBody, err := h.fetchPinHTML()
	if err != nil {
		slog.Error("Failed to fetch Pinterest pin page", "Post", h.postID, "Error", err.Error())
		return nil, "", nil
	}

	pinData := h.extractPinData(htmlBody)
	if pinData == nil {
		slog.Error("Failed to extract Pinterest pin data", "Post", h.postID)
		return nil, "", nil
	}

	if medias, cleanup := h.processPinMedia(pinData); medias != nil {
		return medias, getCaption(pinData), cleanup
	}

	return nil, "", nil
}

func (h *Handler) fetchPinHTML() ([]byte, error) {
	response, err := utils.Request(
		fmt.Sprintf("https://www.pinterest.com/pin/%s/", h.postID),
		utils.RequestParams{
			Method:  "GET",
			Headers: downloader.GenericHeaders,
		},
	)
	if err != nil || response == nil || response.Body == nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func (h *Handler) extractPinData(htmlBody []byte) *PinData {
	matches := relayDataRegex.FindAllSubmatch(htmlBody, -1)
	if len(matches) == 0 {
		slog.Error("No __PWS_RELAY_REGISTER_COMPLETED_REQUEST__ found in HTML", "Post", h.postID)
		return nil
	}

	merged := &PinData{}
	found := false

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		var relay RelayResponse
		if err := json.Unmarshal(match[1], &relay); err != nil {
			slog.Error("Failed to unmarshal Pinterest relay data", "Post", h.postID, "Error", err.Error())
			continue
		}

		for _, query := range relay.Data {
			d := query.Data
			mergePinData(merged, &d)
			found = true
		}
	}

	if !found {
		return nil
	}

	return merged
}

func mergePinData(dst, src *PinData) {
	if src.Description != "" {
		dst.Description = src.Description
	}
	if src.Title != "" {
		dst.Title = src.Title
	}
	if src.GridTitle != "" {
		dst.GridTitle = src.GridTitle
	}
	if src.CloseupUnifiedTitle != "" {
		dst.CloseupUnifiedTitle = src.CloseupUnifiedTitle
	}
	if src.ImageLargeURL != "" {
		dst.ImageLargeURL = src.ImageLargeURL
	}
	if src.ImagesOrig != nil && src.ImagesOrig.URL != "" {
		dst.ImagesOrig = src.ImagesOrig
	}
	if src.Pinner != nil && src.Pinner.Username != "" {
		dst.Pinner = src.Pinner
	}
	if src.NativeCreator != nil && src.NativeCreator.Username != "" {
		dst.NativeCreator = src.NativeCreator
	}
	if src.Videos != nil {
		dst.Videos = src.Videos
	}
	if src.CarouselData != nil && len(src.CarouselData.CarouselSlots) > 0 {
		dst.CarouselData = src.CarouselData
	}
	if src.StoryPinData != nil && len(src.StoryPinData.Pages) > 0 {
		if dst.StoryPinData == nil {
			dst.StoryPinData = src.StoryPinData
		} else {
			for i, srcPage := range src.StoryPinData.Pages {
				if i < len(dst.StoryPinData.Pages) {
					dstPage := &dst.StoryPinData.Pages[i]
					if len(srcPage.Blocks) > len(dstPage.Blocks) {
						*dstPage = srcPage
					}
				} else {
					dst.StoryPinData.Pages = append(dst.StoryPinData.Pages, srcPage)
				}
			}
		}
	}
}

func (h *Handler) processPinMedia(pinData *PinData) ([]gotgbot.InputMedia, func()) {
	if pinData.StoryPinData != nil && len(pinData.StoryPinData.Pages) > 0 {
		return h.handleStoryPin(pinData)
	}

	if pinData.CarouselData != nil && len(pinData.CarouselData.CarouselSlots) > 0 {
		return h.handleCarousel(pinData)
	}

	if pinData.Videos != nil {
		return h.handleVideo(pinData)
	}

	return h.handleImage(pinData)
}

func (h *Handler) handleVideo(pinData *PinData) ([]gotgbot.InputMedia, func()) {
	video := pickBestVideo(pinData.Videos)
	if video == nil {
		return nil, nil
	}

	if strings.HasSuffix(video.URL, ".m3u8") {
		return h.handleHLSVideo(video)
	}

	stream, cleanup, err := downloader.FetchStreamFromURL(video.URL)
	if err != nil {
		slog.Error("Failed to download Pinterest video", "Post", h.postID, "Error", err.Error())
		return nil, nil
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%s", h.postID))
	media := &gotgbot.InputMediaVideo{
		Media:             downloader.InputFileFromReader(filename, stream),
		SupportsStreaming: true,
		Width:             int64(video.Width),
		Height:            int64(video.Height),
		Duration:          int64(video.Duration),
	}

	if video.Thumbnail != "" {
		thumbnail, err := downloader.FetchBytesFromURL(video.Thumbnail)
		if err == nil {
			thumbnail, err = utils.ResizeThumbnail(thumbnail)
			if err != nil {
				slog.Error("Failed to resize thumbnail", "Post", h.postID, "Error", err.Error())
			}
			if thumbnail != nil {
				media.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(thumbnail))
			}
		}
	}

	return []gotgbot.InputMedia{media}, cleanup
}

func (h *Handler) handleHLSVideo(video *VideoVariant) ([]gotgbot.InputMedia, func()) {
	file, cleanup, err := downloader.FetchM3U8ToFile(video.URL, nil, nil)
	if err != nil {
		slog.Error("Failed to download Pinterest HLS video", "Post", h.postID, "Error", err.Error())
		return nil, nil
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%s", h.postID))
	media := &gotgbot.InputMediaVideo{
		Media:             downloader.InputFileFromReader(filename, file),
		SupportsStreaming: true,
		Width:             int64(video.Width),
		Height:            int64(video.Height),
		Duration:          int64(video.Duration),
	}

	if video.Thumbnail != "" {
		thumbnail, err := downloader.FetchBytesFromURL(video.Thumbnail)
		if err == nil {
			thumbnail, err = utils.ResizeThumbnail(thumbnail)
			if err != nil {
				slog.Error("Failed to resize thumbnail", "Post", h.postID, "Error", err.Error())
			}
			if thumbnail != nil {
				media.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(thumbnail))
			}
		}
	}

	return []gotgbot.InputMedia{media}, cleanup
}

func (h *Handler) handleImage(pinData *PinData) ([]gotgbot.InputMedia, func()) {
	imageURL := ""
	if pinData.ImagesOrig != nil && pinData.ImagesOrig.URL != "" {
		imageURL = pinData.ImagesOrig.URL
	} else if pinData.ImageLargeURL != "" {
		imageURL = pinData.ImageLargeURL
	}

	if imageURL == "" {
		return nil, nil
	}

	stream, cleanup, err := downloader.FetchStreamFromURL(imageURL)
	if err != nil {
		slog.Error("Failed to download Pinterest image", "Post", h.postID, "Error", err.Error())
		return nil, nil
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%s", h.postID))
	return []gotgbot.InputMedia{&gotgbot.InputMediaPhoto{
		Media: downloader.InputFileFromReader(filename, stream),
	}}, cleanup
}

func (h *Handler) handleStoryPin(pinData *PinData) ([]gotgbot.InputMedia, func()) {
	var allBlocks []StoryPinBlock
	for _, page := range pinData.StoryPinData.Pages {
		allBlocks = append(allBlocks, page.Blocks...)
	}

	if len(allBlocks) == 0 {
		return nil, nil
	}

	type mediaResult struct {
		index   int
		media   gotgbot.InputMedia
		cleanup func()
		err     error
	}

	results := make(chan mediaResult, len(allBlocks))
	mediaItems := make([]gotgbot.InputMedia, len(allBlocks))
	var cleanups []func()
	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	for i, block := range allBlocks {
		go func(index int, block StoryPinBlock) {
			switch block.Typename {
			case "StoryPinVideoBlock":
				video := h.extractStoryVideoVariant(block.VideoDataV2)
				if video == nil {
					results <- mediaResult{index: index, err: fmt.Errorf("no video variant found")}
					return
				}

				var stream io.ReadCloser
				var cleanup func()
				var err error
				if strings.HasSuffix(video.URL, ".m3u8") {
					file, fileCleanup, fileErr := downloader.FetchM3U8ToFile(video.URL, nil, nil)
					if fileErr != nil {
						results <- mediaResult{index: index, err: fileErr}
						return
					}
					results <- mediaResult{index: index, media: &gotgbot.InputMediaVideo{
						Media:             downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%d_%s", index, h.postID)), file),
						SupportsStreaming: true,
						Width:             int64(video.Width),
						Height:            int64(video.Height),
						Duration:          int64(video.Duration),
					}, cleanup: fileCleanup}
					return
				} else {
					stream, cleanup, err = downloader.FetchStreamFromURL(video.URL)
				}
				if err != nil {
					results <- mediaResult{index: index, err: err}
					return
				}

				filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%d_%s", index, h.postID))
				media := &gotgbot.InputMediaVideo{
					Media:             downloader.InputFileFromReader(filename, stream),
					SupportsStreaming: true,
					Width:             int64(video.Width),
					Height:            int64(video.Height),
					Duration:          int64(video.Duration),
				}

				if video.Thumbnail != "" {
					thumbnail, thumbErr := downloader.FetchBytesFromURL(video.Thumbnail)
					if thumbErr == nil {
						thumbnail, _ = utils.ResizeThumbnail(thumbnail)
						if thumbnail != nil {
							media.Thumbnail = downloader.InputFileFromReader(filename, bytes.NewReader(thumbnail))
						}
					}
				}

				results <- mediaResult{index: index, media: media, cleanup: cleanup}

			case "StoryPinImageBlock":
				var imageURL string
				if block.ImageData != nil && block.ImageData.Images.Orig != nil {
					imageURL = block.ImageData.Images.Orig.URL
				}
				if imageURL == "" {
					results <- mediaResult{index: index, err: fmt.Errorf("no image URL found")}
					return
				}

				stream, cleanup, err := downloader.FetchStreamFromURL(imageURL)
				if err != nil {
					results <- mediaResult{index: index, err: err}
					return
				}

				filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%d_%s", index, h.postID))
				results <- mediaResult{index: index, media: &gotgbot.InputMediaPhoto{
					Media: downloader.InputFileFromReader(filename, stream),
				}, cleanup: cleanup}

			default:
				results <- mediaResult{index: index, err: fmt.Errorf("unsupported block type: %s", block.Typename)}
			}
		}(i, block)
	}

	for range allBlocks {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download story pin media", "Post", h.postID, "Index", result.index, "Error", result.err.Error())
			continue
		}
		addCleanup(result.cleanup)
		mediaItems[result.index] = result.media
	}

	filtered := make([]gotgbot.InputMedia, 0, len(mediaItems))
	for _, item := range mediaItems {
		if item != nil {
			filtered = append(filtered, item)
		}
	}

	if len(filtered) == 0 {
		return nil, downloader.CombineCleanups(cleanups...)
	}

	return filtered, downloader.CombineCleanups(cleanups...)
}

func (h *Handler) handleCarousel(pinData *PinData) ([]gotgbot.InputMedia, func()) {
	slots := pinData.CarouselData.CarouselSlots

	type mediaResult struct {
		index   int
		media   gotgbot.InputMedia
		cleanup func()
		err     error
	}

	results := make(chan mediaResult, len(slots))
	mediaItems := make([]gotgbot.InputMedia, len(slots))
	var cleanups []func()
	addCleanup := func(cleanup func()) {
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}

	for i, slot := range slots {
		go func(index int, slot CarouselSlot) {
			if slot.Videos != nil {
				video := pickBestVideo(slot.Videos)
				if video != nil {
					var stream io.ReadCloser
					var cleanup func()
					var err error
					if strings.HasSuffix(video.URL, ".m3u8") {
						file, fileCleanup, fileErr := downloader.FetchM3U8ToFile(video.URL, nil, nil)
						if fileErr != nil {
							results <- mediaResult{index: index, err: fileErr}
							return
						}
						filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%d_%s", index, h.postID))
						results <- mediaResult{index: index, media: &gotgbot.InputMediaVideo{
							Media:             downloader.InputFileFromReader(filename, file),
							SupportsStreaming: true,
							Width:             int64(video.Width),
							Height:            int64(video.Height),
							Duration:          int64(video.Duration),
						}, cleanup: fileCleanup}
						return
					} else {
						stream, cleanup, err = downloader.FetchStreamFromURL(video.URL)
					}
					if err != nil {
						results <- mediaResult{index: index, err: err}
						return
					}

					filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%d_%s", index, h.postID))
					results <- mediaResult{index: index, media: &gotgbot.InputMediaVideo{
						Media:             downloader.InputFileFromReader(filename, stream),
						SupportsStreaming: true,
						Width:             int64(video.Width),
						Height:            int64(video.Height),
						Duration:          int64(video.Duration),
					}, cleanup: cleanup}
					return
				}
			}

			imageURL := ""
			if slot.ImagesOrig != nil && slot.ImagesOrig.URL != "" {
				imageURL = slot.ImagesOrig.URL
			}
			if imageURL == "" {
				results <- mediaResult{index: index, err: fmt.Errorf("no media found in carousel slot")}
				return
			}

			stream, cleanup, err := downloader.FetchStreamFromURL(imageURL)
			if err != nil {
				results <- mediaResult{index: index, err: err}
				return
			}

			filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-Pinterest_%d_%s", index, h.postID))
			results <- mediaResult{index: index, media: &gotgbot.InputMediaPhoto{
				Media: downloader.InputFileFromReader(filename, stream),
			}, cleanup: cleanup}
		}(i, slot)
	}

	for range slots {
		result := <-results
		if result.err != nil {
			slog.Error("Failed to download carousel media", "Post", h.postID, "Index", result.index, "Error", result.err.Error())
			continue
		}
		addCleanup(result.cleanup)
		mediaItems[result.index] = result.media
	}

	filtered := make([]gotgbot.InputMedia, 0, len(mediaItems))
	for _, item := range mediaItems {
		if item != nil {
			filtered = append(filtered, item)
		}
	}

	if len(filtered) == 0 {
		return nil, downloader.CombineCleanups(cleanups...)
	}

	return filtered, downloader.CombineCleanups(cleanups...)
}

func pickBestVideo(videos *PinVideos) *VideoVariant {
	candidates := []*VideoVariant{
		videos.VideoList.V1080P,
		videos.VideoList.V720P,
		videos.VideoList.V480P,
		videos.VideoList.V360P,
		videos.VideoList.V240P,
		videos.VideoList.VEXP7,
		videos.VideoList.VEXP6,
		videos.VideoList.VEXP5,
		videos.VideoList.VEXP4,
		videos.VideoList.VEXP3,
		videos.VideoList.VEXP2,
		videos.VideoList.VEXP1,
		videos.VideoList.VEXP0,
		videos.VideoList.VHLSV4,
		videos.VideoList.VHLSV3,
	}

	for _, v := range candidates {
		if v != nil && v.URL != "" {
			return v
		}
	}

	return nil
}

func (h *Handler) extractStoryVideoVariant(vData *VideoDataV2) *VideoVariant {
	if vData == nil {
		return nil
	}

	candidates := []*VideoVariant{}
	if vData.VideoList720P != nil && vData.VideoList720P.V720P != nil {
		candidates = append(candidates, vData.VideoList720P.V720P)
	}
	if vData.VHLSV4VideoList != nil && vData.VHLSV4VideoList.VHLSV4 != nil {
		candidates = append(candidates, vData.VHLSV4VideoList.VHLSV4)
	}
	if vData.VideoListMobile != nil && vData.VideoListMobile.VHLSV3Mobile != nil {
		candidates = append(candidates, vData.VideoListMobile.VHLSV3Mobile)
	}
	if vData.VideoList != nil && vData.VideoList.VHLSV3Mobile != nil {
		candidates = append(candidates, vData.VideoList.VHLSV3Mobile)
	}

	for _, v := range candidates {
		if v != nil && v.URL != "" {
			return v
		}
	}

	return nil
}

func getCaption(pinData *PinData) string {
	var sb strings.Builder

	username := ""
	if pinData.Pinner != nil && pinData.Pinner.Username != "" {
		username = pinData.Pinner.Username
	} else if pinData.NativeCreator != nil && pinData.NativeCreator.Username != "" {
		username = pinData.NativeCreator.Username
	}

	if username != "" {
		escapedUsername := html.EscapeString(username)
		fmt.Fprintf(&sb, "<a href='pinterest.com/%s'><b>%s</b></a>", escapedUsername, escapedUsername)
	}

	title := pinData.CloseupUnifiedTitle
	if title == "" {
		title = pinData.GridTitle
	}
	if title == "" {
		title = pinData.Title
	}

	if title != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(html.EscapeString(title))
	}

	if pinData.Description != "" && pinData.Description != title {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(html.EscapeString(pinData.Description))
	}

	return sb.String()
}
