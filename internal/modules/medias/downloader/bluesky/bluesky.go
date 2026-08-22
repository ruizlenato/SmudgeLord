// Package bluesky implements a Bluesky media downloader.
package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/grafov/m3u8"

	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func Handle(text string, _ downloader.Options) downloader.PostInfo {
	handler := &Handler{}
	if !handler.setPostID(text) {
		return downloader.PostInfo{}
	}

	postInfo, err := downloader.GetMediaCache(handler.postID)
	if err == nil {
		return postInfo
	}

	blueskyData := handler.getBlueskyData()
	if blueskyData == nil {
		return downloader.PostInfo{}
	}

	medias, cleanup := handler.processMedia(blueskyData)

	postInfo = downloader.PostInfo{
		ID:      handler.postID,
		Medias:  medias,
		Caption: getCaption(blueskyData),
	}
	postInfo.Cleanup = downloader.CombineCleanups(postInfo.Cleanup, cleanup)

	if blueskyData.Thread.Post.Embed.Record != nil {
		article, articleCleanup := handler.buildArticle(blueskyData, postInfo.Medias)
		if article != nil {
			postInfo.Article = article
			postInfo.Cleanup = downloader.CombineCleanups(postInfo.Cleanup, articleCleanup)
		}
	}

	if len(postInfo.Medias) == 0 && postInfo.Article == nil {
		return downloader.NewNoMediaPostInfo(handler.postID)
	}

	return postInfo
}

func (h *Handler) setPostID(url string) bool {
	if matches := regexp.MustCompile(`([^/?#]+)/post/([A-Za-z0-9_-]+)`).FindStringSubmatch(url); len(matches) == 3 {
		h.username = matches[1]
		h.postID = matches[2]
		return true
	}

	return false
}

func (h *Handler) getBlueskyData() BlueskyData {
	response, err := utils.Request("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread", utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent":   downloader.GenericHeaders["User-Agent"],
			"Content-Type": "application/json",
		},
		Query: map[string]string{
			"uri":   fmt.Sprintf("at://%s/app.bsky.feed.post/%s", h.username, h.postID),
			"depth": "0",
		},
	})

	if err != nil || response.Body == nil {
		return nil
	}
	defer response.Body.Close()

	var data BlueskyData
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		slog.Error("Failed to unmarshal JSON",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil
	}

	return data
}

func getCaption(bluesky BlueskyData) string {
	return fmt.Sprintf("<b>%s (<code>%s</code>)</b>:\n%s",
		html.EscapeString(bluesky.Thread.Post.Author.DisplayName),
		html.EscapeString(bluesky.Thread.Post.Author.Handle),
		html.EscapeString(bluesky.Thread.Post.Record.Text))
}

func (h *Handler) processMedia(data BlueskyData) ([]gotgbot.InputMedia, func()) {
	switch {
	case strings.Contains(data.Thread.Post.Embed.Type, "image"):
		return h.handleImage(data.Thread.Post.Embed.Images)
	case strings.Contains(data.Thread.Post.Embed.Type, "gallery"):
		return h.handleImage(data.Thread.Post.Embed.Items)
	case strings.Contains(data.Thread.Post.Embed.Type, "video"):
		return h.handleVideo(data)
	case strings.Contains(data.Thread.Post.Embed.Type, "recordWithMedia"):
		if strings.Contains(data.Thread.Post.Embed.Media.Type, "image") {
			return h.handleImage(data.Thread.Post.Embed.Media.Images)
		}
		if strings.Contains(data.Thread.Post.Embed.Media.Type, "gallery") {
			return h.handleImage(data.Thread.Post.Embed.Media.Items)
		}
		if strings.Contains(data.Thread.Post.Embed.Media.Type, "video") {
			return h.handleVideo(data)
		}
		return nil, nil
	default:
		return nil, nil
	}
}

func parseResolution(resolution string) (int, int, error) {
	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid resolution format: %s", resolution)
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %s", parts[0])
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %s", parts[1])
	}

	return width, height, nil
}

func getPlaylistAndThumbnailURLs(data BlueskyData) (string, string) {
	if data.Thread.Post.Embed.Playlist != nil {
		return *data.Thread.Post.Embed.Playlist, *data.Thread.Post.Embed.Thumbnail
	}
	return data.Thread.Post.Embed.Media.Playlist, data.Thread.Post.Embed.Media.Thumbnail
}

func (h *Handler) handleVideo(data BlueskyData) ([]gotgbot.InputMedia, func()) {
	playlistURL, thumbnailURL := getPlaylistAndThumbnailURLs(data)
	if playlistURL == "" || thumbnailURL == "" {
		return nil, nil
	}
	return h.handleVideoFromURLs(playlistURL, thumbnailURL)
}

func (h *Handler) handleVideoFromURLs(playlistURL, thumbnailURL string) ([]gotgbot.InputMedia, func()) {
	if !strings.HasPrefix(playlistURL, "https://video.bsky.app/") {
		return nil, nil
	}

	response, err := utils.Request(playlistURL, utils.RequestParams{
		Method: "GET",
	})
	if err != nil {
		slog.Error("Failed to request playlist",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}
	defer response.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(response.Body, true)
	if err != nil {
		slog.Error("Failed to decode m3u8 playlist",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	if listType != m3u8.MASTER {
		return nil, nil
	}

	var highestBandwidthVariant *m3u8.Variant
	for _, variant := range playlist.(*m3u8.MasterPlaylist).Variants {
		if highestBandwidthVariant == nil || variant.Bandwidth > highestBandwidthVariant.Bandwidth {
			highestBandwidthVariant = variant
		}
	}

	url := fmt.Sprintf("%s://%s%s/%s",
		string(response.Request.URL.Scheme),
		string(response.Request.URL.Host),
		path.Dir(string(response.Request.URL.Path)),
		highestBandwidthVariant.URI)

	width, height, err := parseResolution(highestBandwidthVariant.Resolution)
	if err != nil {
		slog.Error("Failed to parse resolution",
			"Post Info", []string{h.username, h.postID},
			"Error", err.Error())
		return nil, nil
	}

	file, cleanup, err := downloader.FetchStreamFromURL(url)
	if err != nil {
		slog.Error("Failed to download video",
			"Post Info", []string{h.username, h.postID},
			"Video URL", url,
			"Error", err.Error())
		return nil, nil
	}

	thumbnail, err := downloader.FetchBytesFromURL(thumbnailURL)
	if err != nil {
		slog.Error("Failed to download thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error())
		return nil, cleanup
	}

	thumbnail, err = utils.ResizeThumbnail(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail",
			"Post Info", []string{h.username, h.postID},
			"Thumbnail URL", thumbnailURL,
			"Error", err.Error())
		return nil, cleanup
	}

	return []gotgbot.InputMedia{&gotgbot.InputMediaVideo{
		Media:                 downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)), file),
		Thumbnail:             downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)), bytes.NewReader(thumbnail)),
		Width:                 int64(width),
		Height:                int64(height),
		SupportsStreaming:     true,
		ShowCaptionAboveMedia: false,
	}}, cleanup
}

func (h *Handler) handleImage(blueskyImages []Image) ([]gotgbot.InputMedia, func()) {
	mediaCount := len(blueskyImages)
	mediaItems := make([]gotgbot.InputMedia, mediaCount)

	results := downloader.DownloadAllMedia(blueskyImages, func(_ int, media Image) (gotgbot.InputMedia, func(), error) {
		file, cleanup, err := downloader.FetchStreamFromURL(media.Fullsize)
		if err != nil {
			slog.Error("Failed to download image",
				"Post Info", []string{h.username, h.postID},
				"Image URL", media.Fullsize,
				"Error", err.Error())
			return nil, cleanup, err
		}
		return &gotgbot.InputMediaPhoto{
			Media: downloader.InputFileFromReader(utils.SanitizeString(fmt.Sprintf("SmudgeLord-Bluesky_%s_%s", h.username, h.postID)), file),
		}, cleanup, nil
	})

	var cleanups []func()
	for _, result := range results {
		if result.Err != nil {
			slog.Error("Failed to download media in carousel",
				"Post Info", []string{h.username, h.postID},
				"Media Count", result.Index,
				"Error", result.Err.Error())
			if result.Cleanup != nil {
				cleanups = append(cleanups, result.Cleanup)
			}
			continue
		}
		if result.Media != nil {
			cleanups = append(cleanups, result.Cleanup)
			mediaItems[result.Index] = result.Media
		}
	}

	nonNil := make([]gotgbot.InputMedia, 0, len(mediaItems))
	for _, m := range mediaItems {
		if m != nil {
			nonNil = append(nonNil, m)
		}
	}

	if len(cleanups) > 0 {
		return nonNil, downloader.CombineCleanups(cleanups...)
	}

	return nonNil, nil
}

func (h *Handler) buildArticle(data BlueskyData, mainMedias []gotgbot.InputMedia) (*downloader.ArticleContent, func()) {
	vr := data.Thread.Post.Embed.Record.ViewRecord()
	if vr == nil {
		return nil, nil
	}

	var htmlBuilder strings.Builder
	var mediaList []gotgbot.InputRichMessageMedia

	writeBlueskyHeaderAndText(&htmlBuilder, data.Thread.Post.Author, data.Thread.Post.Record.Text)

	if len(mainMedias) > 0 {
		mediaList = downloader.AppendRichMedia(&htmlBuilder, mainMedias, 1)
	}

	htmlBuilder.WriteString("<blockquote>\n")

	writeBlueskyHeaderAndText(&htmlBuilder, vr.Author, vr.Value.Text)

	quoteMedias, quoteCleanup := h.downloadQuoteMedia(vr.Embeds)
	if len(quoteMedias) > 0 {
		mediaList = append(mediaList, downloader.AppendRichMedia(&htmlBuilder, quoteMedias, len(mediaList)+1)...)
	}

	htmlBuilder.WriteString("</blockquote>\n")

	if len(mediaList) == 0 {
		return nil, nil
	}

	return &downloader.ArticleContent{HTML: htmlBuilder.String(), Media: mediaList}, quoteCleanup
}

func writeBlueskyHeaderAndText(sb *strings.Builder, author Author, text string) {
	displayName := html.EscapeString(author.DisplayName)
	if displayName == "" {
		displayName = html.EscapeString(author.Handle)
	}
	fmt.Fprintf(sb, "<p><b><a href=\"https://bsky.app/profile/%s\">%s</a> (<code>@%s</code>)</b></p>\n",
		html.EscapeString(author.Handle), displayName, html.EscapeString(author.Handle))
	downloader.AppendCaptionParagraph(sb, html.EscapeString(text))
}

func (h *Handler) downloadQuoteMedia(embeds []QuoteEmbed) ([]gotgbot.InputMedia, func()) {
	var allMedias []gotgbot.InputMedia
	var cleanups []func()

	for _, embed := range embeds {
		if strings.Contains(embed.Type, "image") && len(embed.Images) > 0 {
			medias, cleanup := h.handleImage(embed.Images)
			allMedias = append(allMedias, medias...)
			if cleanup != nil {
				cleanups = append(cleanups, cleanup)
			}
		}
		if strings.Contains(embed.Type, "gallery") && len(embed.Items) > 0 {
			medias, cleanup := h.handleImage(embed.Items)
			allMedias = append(allMedias, medias...)
			if cleanup != nil {
				cleanups = append(cleanups, cleanup)
			}
		}
		if strings.Contains(embed.Type, "recordWithMedia") {
			if strings.Contains(embed.Media.Type, "image") && len(embed.Media.Images) > 0 {
				medias, cleanup := h.handleImage(embed.Media.Images)
				allMedias = append(allMedias, medias...)
				if cleanup != nil {
					cleanups = append(cleanups, cleanup)
				}
			}
			if strings.Contains(embed.Media.Type, "gallery") && len(embed.Media.Items) > 0 {
				medias, cleanup := h.handleImage(embed.Media.Items)
				allMedias = append(allMedias, medias...)
				if cleanup != nil {
					cleanups = append(cleanups, cleanup)
				}
			}
			if strings.Contains(embed.Media.Type, "video") && embed.Media.Playlist != "" {
				medias, cleanup := h.handleVideoFromURLs(embed.Media.Playlist, embed.Media.Thumbnail)
				allMedias = append(allMedias, medias...)
				if cleanup != nil {
					cleanups = append(cleanups, cleanup)
				}
			}
		}
		if strings.Contains(embed.Type, "video") && embed.Playlist != "" {
			medias, cleanup := h.handleVideoFromURLs(embed.Playlist, embed.Thumbnail)
			allMedias = append(allMedias, medias...)
			if cleanup != nil {
				cleanups = append(cleanups, cleanup)
			}
		}
	}

	return allMedias, downloader.CombineCleanups(cleanups...)
}
