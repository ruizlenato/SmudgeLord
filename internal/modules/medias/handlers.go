package medias

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	youtubedl "github.com/kkdai/youtube/v2"

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

type MediaHandler struct {
	Name    string
	Handler func(string) downloader.PostInfo
}

var mediaHandlers = map[string]MediaHandler{
	"bsky.app/":                  {"BlueSky", bluesky.Handle},
	"instagram.com/":             {"Instagram", instagram.Handle},
	"reddit.com/":                {"Reddit", reddit.Handle},
	"threads.com/":               {"Threads", threads.Handle},
	"tiktok.com/":                {"TikTok", tiktok.Handle},
	"(twitter|x).com/":           {"Twitter/X", twitter.Handle},
	"(xiaohongshu|xhslink).com/": {"XiaoHongShu", xiaohongshu.Handle},
	"youtube.com/":               {"YouTube", youtube.Handle},
}

func extractURL(text string) (string, bool) {
	url := regexp.MustCompile(regexMedia).FindStringSubmatch(text)
	if len(url) < 1 {
		return "", false
	}
	return url[0], true
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

func shouldProcessMedia(message *models.Message) bool {
	if regexp.MustCompile(`^/dl`).MatchString(message.Text) || message.Chat.Type == models.ChatTypePrivate {
		return true
	}

	var mediasAuto bool
	if err := database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", message.Chat.ID).Scan(&mediasAuto); err != nil || !mediasAuto {
		return false
	}
	return true
}

func prepareCaption(postInfo *downloader.PostInfo, url string, i18n func(string, ...map[string]any) string) bool {
	if len(postInfo.Medias) == 0 {
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

func setMediaCaption(media models.InputMedia, caption string) {
	switch v := media.(type) {
	case *models.InputMediaPhoto:
		v.Caption = caption
		v.ParseMode = models.ParseModeHTML
	case *models.InputMediaVideo:
		v.Caption = caption
		v.ParseMode = models.ParseModeHTML
	}
}

func getInputFile(media string, attachment io.Reader) models.InputFile {
	if attachment != nil {
		return &models.InputFileUpload{
			Filename: media,
			Data:     attachment,
		}
	}
	return &models.InputFileString{
		Data: media,
	}
}

func sendSingleMedia(
	ctx context.Context,
	b *bot.Bot,
	chatID int64,
	media models.InputMedia,
	replyParameters *models.ReplyParameters,
	replyMarkup *models.InlineKeyboardMarkup,
) (any, error) {
	switch media := media.(type) {
	case *models.InputMediaVideo:
		return b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID:                chatID,
			Video:                 getInputFile(media.Media, media.MediaAttachment),
			Thumbnail:             media.Thumbnail,
			Caption:               media.Caption,
			ParseMode:             models.ParseModeHTML,
			ShowCaptionAboveMedia: media.ShowCaptionAboveMedia,
			ReplyParameters:       replyParameters,
			ReplyMarkup:           replyMarkup,
		})
	case *models.InputMediaPhoto:
		return b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:                chatID,
			Photo:                 getInputFile(media.Media, media.MediaAttachment),
			Caption:               media.Caption,
			ParseMode:             models.ParseModeHTML,
			ShowCaptionAboveMedia: media.ShowCaptionAboveMedia,
			ReplyParameters:       replyParameters,
			ReplyMarkup:           replyMarkup,
		})
	default:
		return nil, fmt.Errorf("unsupported media type")
	}
}

func sendMediaAndHandleCaption(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
	postInfo downloader.PostInfo,
	url string,
	i18n func(string, ...map[string]any) string,
) ([]*models.Message, error) {
	if _, err := b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: update.Message.Chat.ID,
		Action: models.ChatActionUploadDocument,
	}); err != nil {
		slog.Warn("Erro ao enviar ação de chat", "error", err)
	}

	setMediaCaption(postInfo.Medias[0], postInfo.Caption)

	var replied any
	var err error

	if len(postInfo.Medias) == 1 {
		replied, err = sendSingleMedia(
			ctx,
			b,
			update.Message.Chat.ID,
			postInfo.Medias[0],
			&models.ReplyParameters{
				MessageID: update.Message.ID,
			},
			&models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{{
					Text: i18n("open-link", map[string]any{
						"service": postInfo.Service,
					}),
					URL: url,
				}}},
			})
	} else {
		replied, err = b.SendMediaGroup(ctx, &bot.SendMediaGroupParams{
			ChatID: update.Message.Chat.ID,
			Media:  postInfo.Medias,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
	}

	if err != nil {
		if strings.Contains(err.Error(), "Bad Request: not enough rights") {
			return nil, nil
		}
		return nil, fmt.Errorf("couldn't send to chat %d: %w", update.Message.Chat.ID, err)
	}

	var sentMessages []*models.Message
	switch v := replied.(type) {
	case *models.Message:
		sentMessages = []*models.Message{v}
	case []*models.Message:
		sentMessages = v
	}

	if update.Message.Chat.Type == models.ChatTypePrivate {
		return sentMessages, nil
	}

	var mediasCaption bool
	if err := database.DB.QueryRow("SELECT mediasCaption FROM chats WHERE id = ?;", update.Message.Chat.ID).Scan(&mediasCaption); err == nil && !mediasCaption {
		lastMessage := sentMessages[len(sentMessages)-1]
		_, err = b.EditMessageCaption(ctx, &bot.EditMessageCaptionParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: lastMessage.ID,
			Caption:   fmt.Sprintf("\n<a href='%s'>🔗 %s</a>", url, i18n("open-link", map[string]any{"service": postInfo.Service})),
			ParseMode: models.ParseModeHTML,
		})
		if err != nil {
			return sentMessages, err
		}
	}

	return sentMessages, nil
}

func mediaDownloadHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if !shouldProcessMedia(update.Message) {
		return
	}

	i18n := localization.Get(update)
	url, found := extractURL(update.Message.Text)
	if !found {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("no-link-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		if err != nil {
			slog.Error("Couldn't send message",
				"Error", err.Error())
			return
		}
	}

	postInfo := processMedia(url)
	if !prepareCaption(&postInfo, url, i18n) {
		return
	}

	var allSentMessages []*models.Message
	totalMedias := len(postInfo.Medias)
	for i := 0; i < totalMedias; i += 10 {
		end := min(i+10, totalMedias)

		batchPostInfo := postInfo
		batchPostInfo.Medias = postInfo.Medias[i:end]

		if i > 0 {
			batchPostInfo.Caption = ""
		}

		sentMessages, err := sendMediaAndHandleCaption(ctx, b, update, batchPostInfo, url, i18n)
		if err != nil {
			if strings.Contains(err.Error(), "too many requests") {
				return
			}
			slog.Error("Couldn't send media batch",
				"Post URL", url,
				"Error", err.Error(),
				"Batch", i/10+1,
			)
			continue
		}
		allSentMessages = append(allSentMessages, sentMessages...)
	}

	if len(allSentMessages) > 0 {
		if err := downloader.SetMediaCache(allSentMessages, postInfo); err != nil {
			slog.Error("Couldn't set media cache",
				"Error", err.Error())
		}
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

	video, err := youtubeClient.GetVideo(videoURL)
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
		map[string]any{
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
	if update.CallbackQuery.Message.Message == nil {
		return
	}
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

func mediasInlineQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.InlineQuery == nil {
		return
	}

	var results []models.InlineQueryResult
	i18n := localization.Get(update)

	url := regexp.MustCompile(regexMedia).FindStringSubmatch(update.InlineQuery.Query)

	switch {
	case len(url) < 1:
		results = []models.InlineQueryResult{
			&models.InlineQueryResultArticle{
				ID:          "unsupported-link",
				Title:       i18n("unsupported-link-title"),
				Description: i18n("unsupported-link-description"),
				InputMessageContent: &models.InputTextMessageContent{
					MessageText: i18n("unsupported-link"),
					ParseMode:   models.ParseModeHTML,
				},
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{{
						{
							Text:         "❌",
							CallbackData: "NONE",
						},
					}},
				},
			},
		}
	default:
		results = []models.InlineQueryResult{
			&models.InlineQueryResultArticle{
				ID:          "media",
				Title:       i18n("click-to-download-media"),
				Description: "Link: " + url[0],
				InputMessageContent: &models.InputTextMessageContent{
					MessageText: "Baixando...",
					ParseMode:   models.ParseModeHTML,
				},
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: [][]models.InlineKeyboardButton{{
						{
							Text:         "⏳",
							CallbackData: "NONE",
						},
					}},
				},
			},
		}
	}

	b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: update.InlineQuery.ID,
		Results:       results,
		CacheTime:     0,
	})
}

func MediasInline(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	inlineResult := update.ChosenInlineResult

	postInfo := processMedia(inlineResult.Query)

	if len(postInfo.Medias) == 0 {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			InlineMessageID: inlineResult.InlineMessageID,
			Text:            i18n("no-media-found"),
			ParseMode:       models.ParseModeHTML,
		})
		return
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
	var err error

	uploadedMsg, err := uploadMediaToLogChannel(ctx, b, postInfo.Medias[0], postInfo.InvertMedia)
	if err != nil {
		slog.Error("Failed to upload media to log channel", "error", err, "query", inlineResult.Query)
		sendInlineError(ctx, b, inlineResult.InlineMessageID, i18n)
		return
	}

	if _, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    uploadedMsg.Chat.ID,
		MessageID: uploadedMsg.ID,
	}); err != nil {
		slog.Warn("Failed to delete temporary message",
			"error", err.Error())
	}

	var mediaToEdit models.InputMedia

	if uploadedMsg.Photo != nil {
		mediaToEdit = &models.InputMediaPhoto{
			Media:                 uploadedMsg.Photo[0].FileID,
			Caption:               postInfo.Caption,
			ParseMode:             models.ParseModeHTML,
			ShowCaptionAboveMedia: postInfo.InvertMedia,
		}
	} else if uploadedMsg.Video != nil {
		mediaToEdit = &models.InputMediaVideo{
			Media:                 uploadedMsg.Video.FileID,
			Caption:               postInfo.Caption,
			ParseMode:             models.ParseModeHTML,
			ShowCaptionAboveMedia: postInfo.InvertMedia,
			SupportsStreaming:     true,
		}
		if uploadedMsg.Video.Thumbnail != nil {
			mediaToEdit.(*models.InputMediaVideo).Thumbnail = &models.InputFileString{
				Data: uploadedMsg.Video.Thumbnail.FileID,
			}
		}
	}

	if _, err = b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		InlineMessageID: inlineResult.InlineMessageID,
		Media:           mediaToEdit,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{
				Text: i18n("open-link", map[string]any{
					"service": postInfo.Service,
				}),
				URL: inlineResult.Query,
			}}},
		},
	}); err != nil {
		slog.Error("Failed to edit inline message media",
			"error", err.Error(),
			"query", inlineResult.Query)
		sendInlineError(ctx, b, inlineResult.InlineMessageID, i18n)
	}
}

func uploadMediaToLogChannel(ctx context.Context, b *bot.Bot, media models.InputMedia, invertMedia bool) (*models.Message, error) {
	switch m := media.(type) {
	case *models.InputMediaPhoto:
		return b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID: config.LogChannelID,
			Photo: &models.InputFileUpload{
				Data: m.MediaAttachment,
			},
			ShowCaptionAboveMedia: invertMedia,
		})
	case *models.InputMediaVideo:
		return b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID: config.LogChannelID,
			Video: &models.InputFileUpload{
				Data: m.MediaAttachment,
			},
			Thumbnail:             m.Thumbnail,
			ShowCaptionAboveMedia: invertMedia,
		})
	default:
		return nil, fmt.Errorf("unsupported media type: %T", media)
	}
}

func sendInlineError(ctx context.Context, b *bot.Bot, inlineMessageID string, i18n func(string, ...map[string]any) string) {
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		InlineMessageID: inlineMessageID,
		Text:            i18n("media-error"),
		ParseMode:       models.ParseModeHTML,
	})
	if err != nil {
		slog.Error("Failed to send error message to inline query", "error", err)
	}
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeInlineQuery, "^http(?:s)?://.+", mediasInlineQuery)
	b.RegisterHandler(bot.HandlerTypeMessageText, regexMedia, mediaDownloadHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "ytdl", youtubeDownloadHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, `^(_(vid|aud))`, youtubeDownloadCallback)

	utils.SaveHelp("medias")
}
