package medias

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/bluesky"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/reddit"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/soundcloud"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/threads"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/tiktok"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/twitter"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/xiaohongshu"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/youtube"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/steino/youtubedl"
)

const (
	regexMedia     = `(?:http(?:s)?://)?(?:m|vm|vt|www|mobile|on)?(?:.)?(?:(?:instagram|twitter|x|tiktok|reddit|soundcloud|bsky|threads|xiaohongshu|xhslink)\.(?:com|net|app)|(?:youtube\.com/)(?:shorts|clip))/(?:\S*)`
	maxSizeCaption = 1024
)

type MediaHandler struct {
	Name    string
	Handler func(string) downloader.PostInfo
}

var mediaHandlers = map[string]MediaHandler{
	"bsky.app/":                  {"BlueSky", bluesky.Handle},
	"instagram.com/":             {"Instagram", instagram.Handle},
	"reddit.com/":                {"Reddit", reddit.Handle},
	"soundcloud.com/":            {"SoundCloud", soundcloud.Handle},
	"threads.net/":               {"Threads", threads.Handle},
	"tiktok.com/":                {"TikTok", tiktok.Handle},
	"(twitter|x).com/":           {"Twitter/X", twitter.Handle},
	"youtube.com/":               {"YouTube", youtube.Handle},
	"(xiaohongshu|xhslink).com/": {"XiaoHongShu", xiaohongshu.Handle},
}

func processMedia(text string) downloader.PostInfo {
	var postInfo downloader.PostInfo

	for pattern, handler := range mediaHandlers {
		if match, _ := regexp.MatchString(pattern, text); match {
			postInfo = handler.Handler(text)
			postInfo.Service = handler.Name
			break
		}
	}

	return postInfo
}

func extractURL(text string) (string, bool) {
	url := regexp.MustCompile(regexMedia).FindStringSubmatch(text)
	if len(url) < 1 {
		return "", false
	}
	return url[0], true
}

func shouldProcessMedia(message *telegram.NewMessage) bool {
	if regexp.MustCompile(`^/dl`).MatchString(message.Text()) || message.ChatType() == telegram.EntityUser {
		return true
	}

	var mediasAuto bool
	if err := database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", message.ChatID()).Scan(&mediasAuto); err != nil || !mediasAuto {
		return false
	}
	return true
}

func validateAndPrepareMedia(message *telegram.NewMessage, postInfo *downloader.PostInfo, url string, i18n func(string, ...map[string]any) string) bool {
	if len(postInfo.Medias) == 0 {
		return false
	}

	if _, InputMediaUploadedPhoto := postInfo.Medias[0].(*telegram.InputMediaUploadedPhoto); len(postInfo.Medias) == 1 &&
		InputMediaUploadedPhoto &&
		message.Media() != nil &&
		message.Media().(*telegram.MessageMediaWebPage) != nil {
		return false
	}

	if utf8.RuneCountInString(postInfo.Caption) > maxSizeCaption {
		postInfo.Caption = downloader.TruncateUTF8Caption(postInfo.Caption,
			url, i18n("open-link", map[string]any{
				"service": postInfo.Service,
			}), len(postInfo.Medias),
		)
	}

	return true
}

func sendMediaAndHandleCaption(message *telegram.NewMessage, postInfo downloader.PostInfo, url string, mediaOptions telegram.MediaOptions, i18n func(string, ...map[string]any) string) error {
	if _, err := message.SendAction("upload_document"); err != nil {
		return err
	}

	var replied any
	var err error
	switch len(postInfo.Medias) {
	case 1:
		replied, err = message.ReplyMedia(postInfo.Medias[0], mediaOptions)
	default:
		replied, err = message.ReplyAlbum(postInfo.Medias, &mediaOptions)
	}
	if err != nil {
		return err
	}

	if err := downloader.SetMediaCache(replied, postInfo); err != nil {
		return err
	}

	if message.ChatType() == telegram.EntityUser {
		return nil
	}

	var mediasCaption bool
	if err := database.DB.QueryRow("SELECT mediasCaption FROM chats WHERE id = ?;", message.ChatID()).Scan(&mediasCaption); err == nil && !mediasCaption {
		var messageToEdit *telegram.NewMessage
		switch v := replied.(type) {
		case []*telegram.NewMessage:
			messageToEdit = v[len(v)-1]
		case *telegram.NewMessage:
			messageToEdit = v
		}

		_, err = messageToEdit.Edit(fmt.Sprintf("\n<a href='%s'>🔗 %s</a>", url, i18n("open-link", map[string]any{"service": postInfo.Service})))
		return err
	}

	return nil
}

func mediasHandler(message *telegram.NewMessage) error {
	if !shouldProcessMedia(message) {
		return nil
	}

	i18n := localization.Get(message)
	url, found := extractURL(message.Text())
	if !found {
		_, err := message.Reply(i18n("no-link-provided"))
		return err
	}

	postInfo := processMedia(url)
	if !validateAndPrepareMedia(message, &postInfo, url, i18n) {
		return nil
	}

	mediaOptions := telegram.MediaOptions{
		InvertMedia: postInfo.InvertMedia,
		Caption:     postInfo.Caption,
		ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
			telegram.ButtonBuilder{}.Row(
				telegram.ButtonBuilder{}.URL(
					i18n("open-link", map[string]any{
						"service": postInfo.Service,
					}),
					url,
				),
			),
		),
	}

	return sendMediaAndHandleCaption(message, postInfo, url, mediaOptions, i18n)
}

func youtubeDownloadHandler(message *telegram.NewMessage) error {
	var videoURL string
	i18n := localization.Get(message)

	if message.IsReply() {
		reply, err := message.GetReplyMessage()
		if err != nil {
			return err
		}
		videoURL = reply.Text()
	} else if len(strings.Fields(message.Text())) > 1 {
		videoURL = strings.Fields(message.Text())[1]
	} else {
		_, err := message.Reply(i18n("youtube-no-url"))
		return err
	}

	ytClient := youtube.ConfigureYoutubeClient()
	video, err := ytClient.GetVideo(videoURL)
	if err != nil {
		_, err := message.Reply(i18n("youtube-invalid-url"))
		return err
	}

	desiredQualityLabels := func(qualityLabel string) bool {
		supportedQualities := []string{"1080p", "720p", "480p", "360p", "240p", "144p"}
		for _, supported := range supportedQualities {
			if strings.Contains(qualityLabel, supported) {
				return true
			}
		}
		return false
	}

	var maxBitrate int
	var maxBitrateIndex int
	for i, format := range video.Formats.Type("video/mp4") {
		if format.Bitrate > maxBitrate && desiredQualityLabels(format.QualityLabel) {
			maxBitrate = format.Bitrate
			maxBitrateIndex = i
		}
	}
	videoStream := video.Formats.Type("video/mp4")[maxBitrateIndex]

	var audioStream youtubedl.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[1]
	}

	text := i18n("youtube-video-info",
		map[string]any{
			"title":     video.Title,
			"author":    video.Author,
			"audioSize": fmt.Sprintf("%.2f", float64(audioStream.ContentLength)/(1024*1024)),
			"videoSize": fmt.Sprintf("%.2f", float64(videoStream.ContentLength+audioStream.ContentLength)/(1024*1024)),
			"duration":  video.Duration.String(),
		})

	keyboard := telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("youtube-download-audio-button"),
				fmt.Sprintf("_aud|%s|%d|%d|%d", video.ID, audioStream.ItagNo, audioStream.ContentLength, message.SenderID()),
			),
			telegram.ButtonBuilder{}.Data(
				i18n("youtube-download-video-button"),
				fmt.Sprintf("_vid|%s|%d|%d|%d", video.ID, videoStream.ItagNo, videoStream.ContentLength+audioStream.ContentLength, message.SenderID()),
			),
		),
	)

	_, err = message.Reply(text, telegram.SendOptions{
		ReplyMarkup: keyboard,
	})

	return err
}

func validateCallback(update *telegram.CallbackQuery, callbackData []string, i18n func(string, ...map[string]any) string) error {
	if userID, _ := strconv.Atoi(callbackData[4]); update.SenderID != int64(userID) {
		_, err := update.Answer(i18n("denied-button-alert"), &telegram.CallbackOptions{
			Alert: true,
		})
		return err
	}

	if size, _ := strconv.ParseInt(callbackData[3], 10, 64); size > int64(1572864000) {
		_, err := update.Answer(i18n("video-exceeds-limit", map[string]any{
			"size": int64(1572864000),
		}), &telegram.CallbackOptions{
			Alert: true,
		})
		return err
	}

	return nil
}

func sendUploadAction(update *telegram.CallbackQuery, downloadType string) error {
	var err error
	switch downloadType {
	case "_aud":
		_, err = update.Client.SendAction(update.Sender.ID, "upload_audio")
	case "_vid":
		_, err = update.Client.SendAction(update.Sender.ID, "upload_video")
	}
	return err
}

func sendMediaResponse(update *telegram.CallbackQuery, outputFile []byte, video *youtubedl.Video, callbackData []string, thumbnail []byte, filename string, mimeType string) error {
	itag, _ := strconv.Atoi(callbackData[2])

	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = mimeType[:idx]
	}

	switch callbackData[0] {
	case "_aud":
		_, err := update.ReplyMedia(outputFile, &telegram.MediaOptions{
			FileName: filename,
			MimeType: mimeType,
			Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeAudio{
				Title:     video.Title,
				Performer: video.Author,
			}},
			Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			Thumb:   thumbnail,
		})
		return err
	case "_vid":
		_, err := update.ReplyMedia(outputFile, &telegram.MediaOptions{
			FileName: filename,
			MimeType: mimeType,
			Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
				SupportsStreaming: true,
				W:                 int32(video.Formats.Itag(itag)[0].Width),
				H:                 int32(video.Formats.Itag(itag)[0].Height),
			}},
			Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			Thumb:   thumbnail,
		})
		return err
	}
	return nil
}

func youtubeDownloadCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	callbackData := strings.Split(update.DataString(), "|")

	if err := validateCallback(update, callbackData, i18n); err != nil {
		return err
	}

	if _, err := update.Edit(i18n("downloading")); err != nil {
		return err
	}

	outputFile, video, mimeType, err := youtube.Downloader(callbackData)
	if err != nil {
		_, err := update.Edit(i18n("youtube-error"))
		return err
	}

	if _, err := update.Edit(i18n("uploading")); err != nil {
		return err
	}

	if err := sendUploadAction(update, callbackData[0]); err != nil {
		return err
	}

	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnail, err := downloader.FetchBytesFromURL(thumbURL)
	if err != nil {
		_, err := update.Edit(i18n("youtube-error"))
		return err
	}

	thumbnail, err = utils.ResizeThumbnailFromBytes(thumbnail)
	if err != nil {
		slog.Error("Failed to resize thumbnail", "Thumbnail URL", thumbURL, "Error", err.Error())
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-%s_%s", video.Author, video.Title))

	if err := sendMediaResponse(update, outputFile, video, callbackData, thumbnail, filename, mimeType); err != nil {
		_, err := update.Edit(i18n("youtube-error"))
		return err
	}

	_, err = update.Delete()
	return err
}

func mediasInlineQuery(i *telegram.InlineQuery) error {
	builder := i.Builder()
	i18n := localization.Get(i)
	url := regexp.MustCompile(regexMedia).FindStringSubmatch(i.Query)

	switch {
	case len(url) < 1:
		builder.Article(i18n("unsupported-link-title"), i18n("unsupported-link-description"), i18n("unsupported-link"), &telegram.ArticleOptions{
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
				telegram.ButtonBuilder{}.Row(
					telegram.ButtonBuilder{}.Data(
						"❌",
						"NONE",
					),
				),
			),
		})
	default:
		builder.Article(i18n("click-to-download-media"), "Link: "+url[0], "Baixando...", &telegram.ArticleOptions{
			ID: "media",
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
				telegram.ButtonBuilder{}.Row(
					telegram.ButtonBuilder{}.Data(
						"⏳",
						"NONE",
					),
				),
			),
		})
	}

	_, err := i.Answer(builder.Results(), telegram.InlineSendOptions{
		CacheTime: 0,
	})
	return err
}

func MediasInline(m *telegram.InlineSend) error {
	i18n := localization.Get(m)

	postInfo := processMedia(m.OriginalUpdate.Query)
	if len(postInfo.Medias) == 0 {
		_, err := m.Edit(i18n("no-media-found"),
			&telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
		return err
	}
	var captionMultipleItems string
	if len(postInfo.Medias) > 1 {
		captionMultipleItems = "\n\n" + i18n("media-multiple-items", map[string]any{
			"count": len(postInfo.Medias),
		}) // Note: The Go Fluent library does not support line breaks at the beginning of strings.
	}

	availableSpace := maxSizeCaption - utf8.RuneCountInString(captionMultipleItems)

	if utf8.RuneCountInString(postInfo.Caption) > availableSpace {
		truncateLength := availableSpace - 3 // "..." length
		if truncateLength > 0 {
			postInfo.Caption = string([]rune(postInfo.Caption)[:truncateLength]) + "..."
		}
	}

	postInfo.Caption += captionMultipleItems

	_, err := m.Edit(postInfo.Caption,
		&telegram.SendOptions{
			InvertMedia: postInfo.InvertMedia,
			Media:       postInfo.Medias[0],
			ParseMode:   telegram.HTML,
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
				telegram.ButtonBuilder{}.Row(
					telegram.ButtonBuilder{}.URL(
						i18n("open-link", map[string]any{
							"service": postInfo.Service,
						}),
						m.OriginalUpdate.Query,
					),
				),
			),
		})
	return err
}

func Load(client *telegram.Client) {
	utils.SotreHelp("medias")
	client.On("message:"+regexMedia, handlers.HandleCommand(mediasHandler))
	client.On("command:dl", handlers.HandleCommand(mediasHandler))
	client.On("command:ytdl", handlers.HandleCommand(youtubeDownloadHandler))
	client.On("callback:^(_(vid|aud))", youtubeDownloadCallback)
	client.On("inline:^http(?:s)?://.+", mediasInlineQuery)

	handlers.DisableableCommands = append(handlers.DisableableCommands, "ytdl", "dl")
}
