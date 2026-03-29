package medias

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	callbackquery "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

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
	"github.com/ruizlenato/youtubedl"
)

const (
	regexMediaGotgbot     = `(?:http(?:s)?://)?(?:m|vm|vt|www|mobile)?(?:.)?(?:(?:instagram|twitter|x|tiktok|reddit|bsky|threads|xiaohongshu|xhslink)\.(?:com|net|app)|youtube\.com/shorts)/(?:\S*)`
	maxSizeCaption        = 1024
	chatActionUploadDoc   = "upload_document"
	chatActionUploadVoice = "upload_voice"
	chatActionUploadVideo = "upload_video"
)

var mediaRegex = regexp.MustCompile(regexMediaGotgbot)

type MediaHandler struct {
	Name    string
	Handler func(string) downloader.PostInfo
}

var mediaHandlers = map[string]MediaHandler{
	"bsky.app/":                  {Name: "BlueSky", Handler: bluesky.Handle},
	"instagram.com/":             {Name: "Instagram", Handler: instagram.Handle},
	"reddit.com/":                {Name: "Reddit", Handler: reddit.Handle},
	"threads.com/":               {Name: "Threads", Handler: threads.Handle},
	"tiktok.com/":                {Name: "TikTok", Handler: tiktok.Handle},
	"(twitter|x).com/":           {Name: "Twitter/X", Handler: twitter.Handle},
	"(xiaohongshu|xhslink).com/": {Name: "XiaoHongShu", Handler: xiaohongshu.Handle},
	"youtube.com/":               {Name: "YouTube", Handler: youtube.Handle},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractURL(text string) (string, bool) {
	match := mediaRegex.FindStringSubmatch(text)
	if len(match) < 1 {
		return "", false
	}
	return match[0], true
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

func shouldProcessMedia(message *gotgbot.Message) bool {
	if message == nil {
		return false
	}

	if strings.HasPrefix(message.Text, "/dl") || message.Chat.Type == gotgbot.ChatTypePrivate {
		return true
	}

	var mediasAuto bool
	if err := database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", message.Chat.Id).Scan(&mediasAuto); err != nil || !mediasAuto {
		return false
	}
	return true
}

func prepareCaption(postInfo *downloader.PostInfo, url string, i18n func(string, ...map[string]any) string) bool {
	if len(postInfo.Medias) == 0 {
		return false
	}

	postInfo.Caption = utils.SanitizeTelegramHTML(postInfo.Caption)
	postInfo.Caption = downloader.TruncateUTF8Caption(postInfo.Caption,
		url, i18n("open-link", map[string]any{"service": postInfo.Service}), len(postInfo.Medias),
	)
	return true
}

func setMediaCaption(media gotgbot.InputMedia, caption string) {
	switch v := media.(type) {
	case *gotgbot.InputMediaPhoto:
		v.Caption = caption
		v.ParseMode = gotgbot.ParseModeHTML
	case *gotgbot.InputMediaVideo:
		v.Caption = caption
		v.ParseMode = gotgbot.ParseModeHTML
	}
}

func openLinkKeyboard(text, url string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: text, Url: url}}}}
}
func sendSingleMedia(
	b *gotgbot.Bot,
	ctx *ext.Context,
	media gotgbot.InputMedia,
	url string,
	buttonText string,
) (*gotgbot.Message, error) {
	if ctx.EffectiveMessage == nil {
		return nil, fmt.Errorf("missing effective message")
	}
	keyboard := openLinkKeyboard(buttonText, url)
	replyParams := &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}
	switch v := media.(type) {
	case *gotgbot.InputMediaPhoto:
		return b.SendPhotoWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, v.Media, &gotgbot.SendPhotoOpts{
			Caption:               v.Caption,
			ParseMode:             v.ParseMode,
			ShowCaptionAboveMedia: v.ShowCaptionAboveMedia,
			ReplyParameters:       replyParams,
			ReplyMarkup:           &keyboard,
		})
	case *gotgbot.InputMediaVideo:
		return b.SendVideoWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, v.Media, &gotgbot.SendVideoOpts{
			Caption:               v.Caption,
			ParseMode:             v.ParseMode,
			ShowCaptionAboveMedia: v.ShowCaptionAboveMedia,
			Width:                 v.Width,
			Height:                v.Height,
			Duration:              v.Duration,
			SupportsStreaming:     v.SupportsStreaming,
			Thumbnail:             v.Thumbnail,
			ReplyParameters:       replyParams,
			ReplyMarkup:           &keyboard,
		})
	default:
		return nil, fmt.Errorf("unsupported media type: %T", media)
	}
}

func sendMediaAndHandleCaption(
	b *gotgbot.Bot,
	ctx *ext.Context,
	postInfo downloader.PostInfo,
	url string,
	i18n func(string, ...map[string]any) string,
) ([]gotgbot.Message, error) {
	if ctx.EffectiveMessage == nil {
		return nil, fmt.Errorf("missing effective message")
	}
	if _, err := b.SendChatActionWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, chatActionUploadDoc, nil); err != nil {
		slog.Warn("failed to send chat action", "error", err)
	}
	setMediaCaption(postInfo.Medias[0], postInfo.Caption)
	var sent []gotgbot.Message
	buttonText := i18n("open-link", map[string]any{"service": postInfo.Service})
	if len(postInfo.Medias) == 1 {
		wrote, err := sendSingleMedia(b, ctx, postInfo.Medias[0], url, buttonText)
		if err != nil {
			return nil, err
		}
		if wrote != nil {
			sent = append(sent, *wrote)
		}
	} else {
		replies, err := b.SendMediaGroupWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, postInfo.Medias, &gotgbot.SendMediaGroupOpts{
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		if err != nil {
			return nil, err
		}
		sent = append(sent, replies...)
	}
	if ctx.EffectiveMessage.Chat.Type != gotgbot.ChatTypePrivate {
		var mediasCaption bool
		if err := database.DB.QueryRow("SELECT mediasCaption FROM chats WHERE id = ?;", ctx.EffectiveMessage.Chat.Id).Scan(&mediasCaption); err == nil && !mediasCaption && len(sent) > 0 {
			last := sent[len(sent)-1]
			_, _, err := b.EditMessageCaptionWithContext(context.Background(), &gotgbot.EditMessageCaptionOpts{
				ChatId:    ctx.EffectiveMessage.Chat.Id,
				MessageId: last.MessageId,
				Caption:   fmt.Sprintf("\n<a href='%s'>🔗 %s</a>", url, buttonText),
				ParseMode: gotgbot.ParseModeHTML,
			})
			if err != nil {
				return sent, err
			}
		}
	}
	return sent, nil
}

func mediaDownloadHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || !shouldProcessMedia(ctx.EffectiveMessage) {
		return nil
	}
	i18n := localization.Get(ctx)
	url, found := extractURL(ctx.EffectiveMessage.Text)
	if !found {
		_, _ = b.SendMessageWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, i18n("no-link-provided"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}
	postInfo := processMedia(url)
	if !prepareCaption(&postInfo, url, i18n) {
		return nil
	}
	var allSent []gotgbot.Message
	for i := 0; i < len(postInfo.Medias); i += 10 {
		end := min(i+10, len(postInfo.Medias))
		batch := postInfo
		batch.Medias = postInfo.Medias[i:end]
		if i > 0 {
			batch.Caption = ""
		}
		sent, err := sendMediaAndHandleCaption(b, ctx, batch, url, i18n)
		if err != nil {
			if strings.Contains(err.Error(), "too many requests") {
				return nil
			}
			slog.Error("Couldn't send media batch", "postUrl", url, "error", err, "batch", i/10+1)
			continue
		}
		allSent = append(allSent, sent...)
	}
	if len(allSent) > 0 {
		if err := downloader.SetMediaCache(allSent, postInfo); err != nil {
			slog.Error("Couldn't set media cache", "error", err)
		}
	}
	return nil
}

func mediasInlineQuery(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.InlineQuery == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	var results []gotgbot.InlineQueryResult
	query := ctx.InlineQuery.Query
	if mediaRegex.MatchString(query) {
		results = []gotgbot.InlineQueryResult{gotgbot.InlineQueryResultArticle{
			Id:          "media",
			Title:       i18n("click-to-download-media"),
			Description: "Link: " + query,
			InputMessageContent: gotgbot.InputTextMessageContent{
				MessageText: "Baixando...",
				ParseMode:   gotgbot.ParseModeHTML,
			},
			ReplyMarkup: &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: "⏳", CallbackData: "NONE"}}}},
		}}
	} else {
		results = []gotgbot.InlineQueryResult{gotgbot.InlineQueryResultArticle{
			Id:          "unsupported-link",
			Title:       i18n("unsupported-link-title"),
			Description: i18n("unsupported-link-description"),
			InputMessageContent: gotgbot.InputTextMessageContent{
				MessageText: i18n("unsupported-link"),
				ParseMode:   gotgbot.ParseModeHTML,
			},
			ReplyMarkup: &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: "❌", CallbackData: "NONE"}}}},
		}}
	}
	cacheTime := int64(0)
	_, _ = b.AnswerInlineQueryWithContext(context.Background(), ctx.InlineQuery.Id, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
	return nil
}

func MediasInline(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.ChosenInlineResult == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	inlineResult := ctx.ChosenInlineResult
	postInfo := processMedia(inlineResult.Query)
	if len(postInfo.Medias) == 0 {
		_, _, _ = b.EditMessageTextWithContext(context.Background(), i18n("no-media-found"), &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineResult.InlineMessageId,
			ParseMode:       gotgbot.ParseModeHTML,
		})
		return nil
	}
	captionMultiple := ""
	if len(postInfo.Medias) > 1 {
		captionMultiple = "\n\n" + i18n("media-multiple-items", map[string]any{"count": len(postInfo.Medias)})
	}
	postInfo.Caption = utils.SanitizeTelegramHTML(postInfo.Caption)
	available := maxSizeCaption - utf8.RuneCountInString(captionMultiple)
	if utf8.RuneCountInString(postInfo.Caption) > available {
		if truncate := available - 3; truncate > 0 {
			postInfo.Caption = string([]rune(postInfo.Caption)[:truncate]) + "..."
		}
	}
	postInfo.Caption += captionMultiple

	uploaded, err := uploadMediaToLogChannel(context.Background(), b, postInfo.Medias[0], postInfo.InvertMedia)
	if err != nil {
		slog.Error("Failed to upload media to log channel", "error", err, "query", inlineResult.Query)
		sendInlineError(ctx, b, inlineResult.InlineMessageId, i18n)
		return nil
	}
	_, _ = b.DeleteMessageWithContext(context.Background(), uploaded.Chat.Id, uploaded.MessageId, nil)
	var media gotgbot.InputMedia
	buttonText := i18n("open-link", map[string]any{"service": postInfo.Service})
	if uploaded.Photo != nil {
		media = &gotgbot.InputMediaPhoto{
			Media:                 gotgbot.InputFileByID(uploaded.Photo[0].FileId),
			Caption:               postInfo.Caption,
			ParseMode:             gotgbot.ParseModeHTML,
			ShowCaptionAboveMedia: postInfo.InvertMedia,
		}
	} else if uploaded.Video != nil {
		media = &gotgbot.InputMediaVideo{
			Media:                 gotgbot.InputFileByID(uploaded.Video.FileId),
			Caption:               postInfo.Caption,
			ParseMode:             gotgbot.ParseModeHTML,
			ShowCaptionAboveMedia: postInfo.InvertMedia,
			SupportsStreaming:     true,
		}
	}
	if media == nil {
		sendInlineError(ctx, b, inlineResult.InlineMessageId, i18n)
		return nil
	}
	_, _, err = b.EditMessageMediaWithContext(context.Background(), media, &gotgbot.EditMessageMediaOpts{
		InlineMessageId: inlineResult.InlineMessageId,
		ReplyMarkup:     openLinkKeyboard(buttonText, inlineResult.Query),
	})
	if err != nil {
		slog.Error("Failed to edit inline message media", "error", err, "query", inlineResult.Query)
		sendInlineError(ctx, b, inlineResult.InlineMessageId, i18n)
	}
	return nil
}

func uploadMediaToLogChannel(ctx context.Context, b *gotgbot.Bot, media gotgbot.InputMedia, invert bool) (*gotgbot.Message, error) {
	switch v := media.(type) {
	case *gotgbot.InputMediaPhoto:
		return b.SendPhotoWithContext(ctx, config.LogChannelID, v.Media, &gotgbot.SendPhotoOpts{ShowCaptionAboveMedia: invert})
	case *gotgbot.InputMediaVideo:
		return b.SendVideoWithContext(ctx, config.LogChannelID, v.Media, &gotgbot.SendVideoOpts{
			SupportsStreaming:     v.SupportsStreaming,
			Thumbnail:             v.Thumbnail,
			ShowCaptionAboveMedia: invert,
		})
	default:
		return nil, fmt.Errorf("unsupported media type: %T", media)
	}
}

func sendInlineError(ctx *ext.Context, b *gotgbot.Bot, inlineID string, i18n func(string, ...map[string]any) string) {
	_, _, _ = b.EditMessageTextWithContext(context.Background(), i18n("media-error"), &gotgbot.EditMessageTextOpts{
		InlineMessageId: inlineID,
		ParseMode:       gotgbot.ParseModeHTML,
	})
}

func youtubeDownloadHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	var videoURL string
	if ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.Text != "" {
		videoURL = ctx.EffectiveMessage.ReplyToMessage.Text
	} else if fields := strings.Fields(ctx.EffectiveMessage.Text); len(fields) > 1 {
		videoURL = fields[1]
	} else {
		_, _ = b.SendMessageWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, i18n("youtube-no-url"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}
	ytClient := youtube.ConfigureYoutubeClient()
	if ytClient == nil {
		return nil
	}
	video, err := ytClient.GetVideo(videoURL)
	if err != nil || video == nil || video.Formats == nil {
		_, _ = b.SendMessageWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, i18n("youtube-invalid-url"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}
	videoStream := youtube.GetBestQualityVideoStream(video.Formats.Type("video/mp4"))
	var audioStream youtubedl.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[0]
	}
	text := i18n("youtube-video-info", map[string]any{
		"title":     video.Title,
		"author":    video.Author,
		"audioSize": fmt.Sprintf("%.2f", float64(audioStream.ContentLength)/(1024*1024)),
		"videoSize": fmt.Sprintf("%.2f", float64(videoStream.ContentLength+audioStream.ContentLength)/(1024*1024)),
		"duration":  video.Duration.String(),
	})
	keyboard := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
		{Text: i18n("youtube-download-audio-button"), CallbackData: fmt.Sprintf("_aud|%s|%d|%d|%d|%d", video.ID, audioStream.ItagNo, audioStream.ContentLength, ctx.EffectiveMessage.MessageId, ctx.EffectiveMessage.From.Id)},
		{Text: i18n("youtube-download-video-button"), CallbackData: fmt.Sprintf("_vid|%s|%d|%d|%d|%d", video.ID, videoStream.ItagNo, videoStream.ContentLength+audioStream.ContentLength, ctx.EffectiveMessage.MessageId, ctx.EffectiveMessage.From.Id)},
	}}}
	_, _ = b.SendMessageWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, text, &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup:     keyboard,
	})
	return nil
}

func youtubeDownloadCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	chat := ctx.CallbackQuery.Message.GetChat()
	messageID := ctx.CallbackQuery.Message.GetMessageId()
	i18n := localization.Get(ctx)
	data := strings.Split(ctx.CallbackQuery.Data, "|")
	if len(data) < 6 {
		_, _ = b.AnswerCallbackQueryWithContext(context.Background(), ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("youtube-error"), ShowAlert: true})
		return nil
	}
	requestedUser, _ := strconv.Atoi(data[5])
	if ctx.CallbackQuery.From.Id != int64(requestedUser) {
		_, _ = b.AnswerCallbackQueryWithContext(context.Background(), ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("denied-button-alert"), ShowAlert: true})
		return nil
	}
	size, _ := strconv.ParseInt(data[3], 10, 64)
	sizeLimit := int64(1572864000)
	if config.BotAPIURL == "" {
		sizeLimit = 52428800
	}
	if size > sizeLimit {
		_, _ = b.AnswerCallbackQueryWithContext(context.Background(), ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("video-exceeds-limit", map[string]any{"size": sizeLimit}), ShowAlert: true})
		return nil
	}
	_, _, _ = b.EditMessageTextWithContext(context.Background(), i18n("downloading"), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: messageID, ParseMode: gotgbot.ParseModeHTML})
	format := "audio"
	if data[0] == "_vid" {
		format = "video"
	}
	cacheFound, cacheErr := trySendCachedYoutubeMedia(ctx, b, data, format)
	if cacheFound && cacheErr == nil {
		_, _ = b.DeleteMessageWithContext(context.Background(), chat.Id, messageID, nil)
		return nil
	}
	fileBytes, video, err := youtube.Downloader(data)
	if err != nil {
		_, _, _ = b.EditMessageTextWithContext(context.Background(), i18n("youtube-error"), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: messageID, ParseMode: gotgbot.ParseModeHTML})
		return nil
	}
	itag, _ := strconv.Atoi(data[2])
	_, _, _ = b.EditMessageTextWithContext(context.Background(), i18n("uploading"), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: messageID, ParseMode: gotgbot.ParseModeHTML})
	action := chatActionUploadVoice
	if data[0] == "_vid" {
		action = chatActionUploadVideo
	}
	_, _ = b.SendChatActionWithContext(context.Background(), chat.Id, action, nil)
	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnailBytes, _ := downloader.FetchBytesFromURL(thumbURL)
	caption := fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title)
	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-%s_%s", video.Author, video.Title))
	var sent *gotgbot.Message
	switch data[0] {
	case "_aud":
		sent, err = b.SendAudioWithContext(context.Background(), chat.Id, gotgbot.InputFileByReader(filename, bytes.NewReader(fileBytes)), &gotgbot.SendAudioOpts{
			Caption:         caption,
			ParseMode:       gotgbot.ParseModeHTML,
			Performer:       video.Author,
			Title:           video.Title,
			Thumbnail:       gotgbot.InputFileByReader(filename, bytes.NewReader(thumbnailBytes)),
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: messageID},
		})
	case "_vid":
		sent, err = b.SendVideoWithContext(context.Background(), chat.Id, gotgbot.InputFileByReader(filename, bytes.NewReader(fileBytes)), &gotgbot.SendVideoOpts{
			Caption:           caption,
			ParseMode:         gotgbot.ParseModeHTML,
			SupportsStreaming: true,
			Width:             int64(video.Formats.Itag(itag)[0].Width),
			Height:            int64(video.Formats.Itag(itag)[0].Height),
			Thumbnail:         gotgbot.InputFileByReader(filename, bytes.NewReader(thumbnailBytes)),
			ReplyParameters:   &gotgbot.ReplyParameters{MessageId: messageID},
		})
	}
	if err != nil {
		slog.Error("Couldn't send media", "error", err)
		_, _, _ = b.EditMessageTextWithContext(context.Background(), i18n("youtube-error"), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: messageID, ParseMode: gotgbot.ParseModeHTML})
		return nil
	}
	_, _ = b.DeleteMessageWithContext(context.Background(), chat.Id, messageID, nil)
	if cacheErr := downloader.SetYoutubeCache(sent, data[1]); cacheErr != nil {
		slog.Error("Couldn't set youtube cache", "error", cacheErr)
	}
	return nil
}

func trySendCachedYoutubeMedia(ctx *ext.Context, b *gotgbot.Bot, data []string, format string) (bool, error) {
	fileID, caption, err := downloader.GetYoutubeCache(data[1], format)
	if err != nil {
		return false, err
	}
	chat := ctx.CallbackQuery.Message.GetChat()
	reply := &gotgbot.ReplyParameters{MessageId: ctx.CallbackQuery.Message.GetMessageId()}
	switch data[0] {
	case "_aud":
		_, err = b.SendAudioWithContext(context.Background(), chat.Id, gotgbot.InputFileByID(fileID), &gotgbot.SendAudioOpts{Caption: caption, ParseMode: gotgbot.ParseModeHTML, ReplyParameters: reply})
	case "_vid":
		_, err = b.SendVideoWithContext(context.Background(), chat.Id, gotgbot.InputFileByID(fileID), &gotgbot.SendVideoOpts{Caption: caption, ParseMode: gotgbot.ParseModeHTML, ReplyParameters: reply})
	}
	return err == nil, err
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewInlineQuery(func(iq *gotgbot.InlineQuery) bool {
		return mediaRegex.MatchString(iq.Query)
	}, mediasInlineQuery))
	dispatcher.AddHandler(handlers.NewMessage(func(m *gotgbot.Message) bool {
		return m != nil && mediaRegex.MatchString(m.Text)
	}, mediaDownloadHandler))
	dispatcher.AddHandler(handlers.NewCommand("ytdl", youtubeDownloadHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("_vid"), youtubeDownloadCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("_aud"), youtubeDownloadCallback))

	utils.SaveHelp("medias")
	utils.DisableableCommands = append(utils.DisableableCommands, "ytdl")
}
