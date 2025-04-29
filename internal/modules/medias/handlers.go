package medias

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/steino/youtubedl"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/bluesky"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/reddit"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/threads"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/tiktok"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/twitter"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/xiaohongshu"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/youtube"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	regexMedia     = `(?:http(?:s)?://)?(?:m|vm|vt|www|mobile)?(?:.)?(?:(?:instagram|twitter|x|tiktok|reddit|bsky|threads|xiaohongshu|xhslink)\.(?:com|net|app)|youtube\.com/shorts)/(?:\S*)`
	maxSizeCaption = 1024
)

func mediaDownloadHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	var (
		mediaItems []models.InputMedia
		result     []string
		caption    string
		forceSend  bool
	)

	if !regexp.MustCompile(`^/(?:s)?dl`).MatchString(update.Message.Text) && update.Message.Chat.Type != models.ChatTypePrivate {
		var mediasAuto bool
		err := database.DB.QueryRow("SELECT mediasAuto FROM groups WHERE id = ?;", update.Message.Chat.ID).Scan(&mediasAuto)
		if err != nil || !mediasAuto {
			return
		}
	}

	url := regexp.MustCompile(regexMedia).FindStringSubmatch(update.Message.Text)
	i18n := localization.Get(update)
	if len(url) < 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("no-link-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	mediaHandlers := map[string]func(string) ([]models.InputMedia, []string){
		"bsky.app/":                  bluesky.Handle,
		"instagram.com/":             instagram.Handle,
		"reddit.com/":                reddit.Handle,
		"threads.net/":               threads.Handle,
		"tiktok.com/":                tiktok.Handle,
		"(twitter|x).com/":           twitter.Handle,
		"(xiaohongshu|xhslink).com/": xiaohongshu.Handle,
	}

	for pattern, handler := range mediaHandlers {
		if match, _ := regexp.MatchString(pattern, update.Message.Text); match {
			if regexp.MustCompile(`(tiktok\.com|reddit\.com)`).MatchString(update.Message.Text) {
				forceSend = true
			}
			mediaItems, result = handler(url[0])
			if len(result) == 2 {
				caption = result[0]
			}
			break

		}
	}

	type mediaInfo struct {
		Type  string `json:"type"`
		Media string `json:"media"`
	}
	var mInfo mediaInfo

	if len(mediaItems) == 0 || mediaItems[0] == nil {
		return
	}

	if len(mediaItems) == 1 && !forceSend &&
		update.Message.LinkPreviewOptions != nil && (update.Message.LinkPreviewOptions.IsDisabled == nil || !*update.Message.LinkPreviewOptions.IsDisabled) {

		marshalInputMedia, err := mediaItems[0].MarshalInputMedia()
		if err != nil {
			slog.Error("Couldn't marshal media info",
				"Error", err.Error())
			return
		}

		err = json.Unmarshal(marshalInputMedia, &mInfo)
		if err != nil {
			slog.Error("Couldn't unmarshal media info",
				"Error", err.Error())
			return
		}

		if mInfo.Type == "photo" {
			return
		}
	}

	if len(mediaItems) > 10 { // Telegram limits up to 10 images and videos in an album.
		mediaItems = mediaItems[:10]
	}

	if utf8.RuneCountInString(caption) > maxSizeCaption {
		caption = downloader.TruncateUTF8Caption(caption, url[0])
	}

	var mediasCaption = true
	if err := database.DB.QueryRow("SELECT mediasCaption FROM groups WHERE id = ?;", update.Message.Chat.ID).Scan(&mediasCaption); err == nil && !mediasCaption || caption == "" {
		caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url[0])
	}

	for _, media := range mediaItems[:1] {
		marshalInputMedia, err := media.MarshalInputMedia()
		if err != nil {
			slog.Error("Couldn't marshal media info",
				"Error", err.Error())
			return
		}

		err = json.Unmarshal(marshalInputMedia, &mInfo)
		if err != nil {
			slog.Error("Couldn't unmarshal media info",
				"Error", err.Error())
			return
		}

		switch mInfo.Type {
		case "photo":
			media.(*models.InputMediaPhoto).Caption = caption
			media.(*models.InputMediaPhoto).ParseMode = models.ParseModeHTML
		case "video":
			media.(*models.InputMediaVideo).Caption = caption
			media.(*models.InputMediaVideo).ParseMode = models.ParseModeHTML
		}
	}

	b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: update.Message.Chat.ID,
		Action: models.ChatActionUploadDocument,
	})

	replied, err := b.SendMediaGroup(ctx, &bot.SendMediaGroupParams{
		ChatID: update.Message.Chat.ID,
		Media:  mediaItems,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
	if err != nil {
		return
	}

	if err := downloader.SetMediaCache(replied, result); err != nil {
		slog.Error("Couldn't set media cache",
			"Error", err.Error())
	}

}

func youtubeDownloadHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	var videoURL string

	if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.Text != "" {
		videoURL = update.Message.ReplyToMessage.Text
	} else if len(strings.Fields(update.Message.Text)) > 1 {
		videoURL = strings.Fields(update.Message.Text)[1]
	} else {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("youtube-no-url"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	youtubeClient := youtube.ConfigureYoutubeClient()
	var video *youtubedl.Video
	var err error

	for attempt := 1; attempt <= 10; attempt++ {
		video, err = youtubeClient.GetVideo(videoURL, youtubedl.WithClient("ANDROID"))
		if err == nil {
			break
		}
		slog.Warn("GetVideo failed, retrying...",
			"attempt", attempt, "error",
			err.Error())
		time.Sleep(5 * time.Second)
	}
	if err != nil || video == nil || video.Formats == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("youtube-invalid-url"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	videoStream := youtube.GetBestQualityVideoStream(video.Formats.Type("video/mp4"))

	var audioStream youtubedl.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[0]
	}

	text := i18n("youtube-video-info",
		map[string]interface{}{
			"title":     video.Title,
			"author":    video.Author,
			"audioSize": fmt.Sprintf("%.2f", float64(audioStream.ContentLength)/(1024*1024)),
			"videoSize": fmt.Sprintf("%.2f", float64(videoStream.ContentLength+audioStream.ContentLength)/(1024*1024)),
			"duration":  video.Duration.String(),
		})

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         i18n("youtube-download-audio-button"),
					CallbackData: fmt.Sprintf("_aud|%s|%d|%d|%d|%d", video.ID, audioStream.ItagNo, audioStream.ContentLength, update.Message.ID, update.Message.From.ID),
				},
				{
					Text:         i18n("youtube-download-video-button"),
					CallbackData: fmt.Sprintf("_vid|%s|%d|%d|%d|%d", video.ID, videoStream.ItagNo, videoStream.ContentLength+audioStream.ContentLength, update.Message.ID, update.Message.From.ID),
				},
			},
		},
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: keyboard,
	})
}

func youtubeDownloadCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	message := update.CallbackQuery.Message.Message
	i18n := localization.Get(update)

	callbackData := strings.Split(update.CallbackQuery.Data, "|")
	if userID, _ := strconv.Atoi(callbackData[5]); update.CallbackQuery.From.ID != int64(userID) {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("denied-button-alert"),
			ShowAlert:       true,
		})
		return
	}

	sizeLimit := int64(1572864000) // 1.5 GB
	if config.BotAPIURL == "" {
		sizeLimit = 52428800 // 50 MB
	}

	if size, _ := strconv.ParseInt(callbackData[3], 10, 64); size > sizeLimit {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text: i18n("video-exceeds-limit", map[string]any{
				"size": sizeLimit,
			}),
			ShowAlert: true,
		})
		return
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("downloading"),
		ParseMode: models.ParseModeHTML,
	})

	messageID, _ := strconv.Atoi(callbackData[4])
	cacheFound, err := trySendCachedYoutubeMedia(ctx, b, update, messageID, callbackData)
	if cacheFound || err == nil {
		b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
		})
		return
	}

	fileBytes, video, err := youtube.Downloader(callbackData)
	if err != nil {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      i18n("youtube-error"),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	itag, _ := strconv.Atoi(callbackData[2])

	var action models.ChatAction
	switch callbackData[0] {
	case "_aud":
		action = models.ChatActionUploadVoice
	case "_vid":
		action = models.ChatActionUploadVideo
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("uploading"),
		ParseMode: models.ParseModeHTML,
	})

	b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: update.CallbackQuery.Message.Message.ID,
		Action: action,
	})

	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnail, _ := downloader.FetchBytesFromURL(thumbURL)

	var replied *models.Message
	caption := fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title)
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-%s_%s", video.Author, video.Title))
	switch callbackData[0] {
	case "_aud":
		replied, err = b.SendAudio(ctx, &bot.SendAudioParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Audio: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewBuffer(fileBytes),
			},
			Caption:   caption,
			Title:     video.Title,
			Performer: video.Author,
			Thumbnail: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewBuffer(thumbnail),
			},
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: messageID,
			},
		})
	case "_vid":
		replied, err = b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Video: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewBuffer(fileBytes),
			},
			Width:  video.Formats.Itag(itag)[0].Width,
			Height: video.Formats.Itag(itag)[0].Height,
			Thumbnail: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewBuffer(thumbnail),
			},
			Caption:           caption,
			ParseMode:         models.ParseModeHTML,
			SupportsStreaming: true,
			ReplyParameters: &models.ReplyParameters{
				MessageID: messageID,
			},
		})
	}
	if err != nil {
		slog.Error("Couldn't send media",
			"Error", err.Error(),
		)
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      i18n("youtube-error"),
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
	})

	if err := downloader.SetYoutubeCache(replied, callbackData[1]); err != nil {
		slog.Error("Couldn't set youtube cache",
			"Error", err.Error())
	}
}

func trySendCachedYoutubeMedia(ctx context.Context, b *bot.Bot, update *models.Update, messageID int, callbackData []string) (bool, error) {
	var fileID, caption string
	var err error

	switch callbackData[0] {
	case "_aud":
		fileID, caption, err = downloader.GetYoutubeCache(callbackData[1], "audio")
	case "_vid":
		fileID, caption, err = downloader.GetYoutubeCache(callbackData[1], "video")
	}

	if err == nil {
		switch callbackData[0] {
		case "_aud":
			b.SendAudio(ctx, &bot.SendAudioParams{
				ChatID:    update.Message.Chat.ID,
				Audio:     &models.InputFileString{Data: fileID},
				Caption:   caption,
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: messageID,
				},
			})
		case "_vid":
			b.SendVideo(ctx, &bot.SendVideoParams{
				ChatID:    update.Message.Chat.ID,
				Video:     &models.InputFileString{Data: fileID},
				Caption:   caption,
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: messageID,
				},
			})
		}
		return true, nil
	}
	return false, err
}

func Load(b *bot.Bot) {
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(regexMedia), mediaDownloadHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "ytdl", bot.MatchTypeCommand, youtubeDownloadHandler)
	b.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, regexp.MustCompile(`^(_(vid|aud))`), youtubeDownloadCallback)

	utils.SaveHelp("medias")
}
