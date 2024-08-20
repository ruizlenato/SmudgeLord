package medias

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/tiktok"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/twitter"
	yt "github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/youtube"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	"github.com/kkdai/youtube/v2"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

const (
	regexMedia     = `(?:http(?:s)?://)?(?:m|vm|www|mobile)?(?:.)?(?:instagram|twitter|x|tiktok|reddit|twitch).(?:com|net|tv)/(?:\S*)`
	maxSizeCaption = 1024
)

func handleMediaDownload(bot *telego.Bot, message telego.Message) {
	var mediaItems []telego.InputMedia
	var caption string

	if !regexp.MustCompile(`^/(?:s)?dl`).MatchString(message.Text) && strings.Contains(message.Chat.Type, "group") {
		var mediasAuto bool
		if err := database.DB.QueryRow("SELECT mediasAuto FROM groups WHERE id = ?;", message.Chat.ID).Scan(&mediasAuto); err != nil || !mediasAuto {
			return
		}
	}

	url := regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text)
	i18n := localization.Get(message.GetChat())
	if len(url) < 1 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("medias.noURL"),
			ParseMode: "HTML",
		})
		return
	}

	mediaHandlers := map[string]func(telego.Message) ([]telego.InputMedia, []string){
		"instagram.com/":   instagram.Handle,
		"(twitter|x).com/": twitter.Handle,
		"tiktok.com/":      tiktok.Handle,
	}

	for pattern, handler := range mediaHandlers {
		if match, _ := regexp.MatchString(pattern, message.Text); match {
			var result []string
			mediaItems, result = handler(message)
			if len(result) == 2 {
				caption = result[0]
			}
			break
		}
	}

	row := database.DB.QueryRow("SELECT mediasCaption FROM groups WHERE id = ?;", message.Chat.ID)
	var mediasCaption bool
	if row.Scan(&mediasCaption); !mediasCaption {
		caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url[0])
	}

	if mediaItems == nil ||
		len(mediaItems) == 0 ||
		len(mediaItems) == 1 &&
			mediaItems[0].MediaType() == "photo" &&
			message.LinkPreviewOptions != nil &&
			!message.LinkPreviewOptions.IsDisabled {
		return
	}

	if mediaItems != nil && caption == "" {
		caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url)
	}

	if utf8.RuneCountInString(caption) > maxSizeCaption {
		caption = downloader.TruncateUTF8Caption(caption,
			regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text)[0])
	}

	for _, media := range mediaItems[:1] {
		switch media.MediaType() {
		case "photo":
			if photo, ok := media.(*telego.InputMediaPhoto); ok {
				photo.WithCaption(caption).WithParseMode("HTML")
			}
		case "video":
			if video, ok := media.(*telego.InputMediaVideo); ok {
				video.WithCaption(caption).WithParseMode("HTML")
			}
		}
	}

	bot.SendChatAction(&telego.SendChatActionParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Action: telego.ChatActionUploadDocument,
	})

	bot.SendMediaGroup(&telego.SendMediaGroupParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Media:  mediaItems,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	downloader.RemoveMediaFiles(mediaItems)
}

func handleYoutubeDownloadCallback(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.GetChat()
	i18n := localization.Get(chat)

	callbackData := strings.Split(update.CallbackQuery.Data, "|")
	if userID, _ := strconv.Atoi(callbackData[5]); update.CallbackQuery.From.ID != int64(userID) {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("medias.youtubeDenied"),
			ShowAlert:       true,
		})
		return
	}

	sizeLimit := int64(1572864000) // 1.5 GB
	if config.BotAPIURL == "" {
		sizeLimit = 52428800 // 50 MB
	}

	if size, _ := strconv.ParseInt(callbackData[3], 10, 64); size > sizeLimit {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("medias.youtubeBigFile"),
			ShowAlert:       true,
		})
		return
	}

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("medias.downloading"),
	})

	outputFile, video, err := yt.Downloader(callbackData)
	if err != nil {
		log.Printf("Failed to youtube download video: %v", err)
		return
	}

	messageID, _ := strconv.Atoi(callbackData[4])
	itag, _ := strconv.Atoi(callbackData[2])

	var action string
	switch callbackData[0] {
	case "_aud":
		action = telego.ChatActionUploadVoice
	case "_vid":
		action = telego.ChatActionUploadVideo
	}

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("medias.uploading"),
	})
	bot.SendChatAction(&telego.SendChatActionParams{
		ChatID: telegoutil.ID(chat.ID),
		Action: action,
	})

	outputFile.Seek(0, 0) // Seek back to the beginning of the file
	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnail, _ := downloader.Downloader(thumbURL)

	defer func() {
		if err := os.Remove(thumbnail.Name()); err != nil {
			log.Printf("Failed to remove thumbnail: %v", err)
		}
	}()

	switch callbackData[0] {
	case "_aud":
		_, err = bot.SendAudio(&telego.SendAudioParams{
			ChatID:    telegoutil.ID(chat.ID),
			Audio:     telegoutil.File(outputFile),
			Thumbnail: &telego.InputFile{File: thumbnail},
			Performer: video.Author,
			Title:     video.Title,
			ReplyParameters: &telego.ReplyParameters{
				MessageID: messageID,
			},
		})
	case "_vid":
		_, err = bot.SendVideo(&telego.SendVideoParams{
			ChatID:            telegoutil.ID(chat.ID),
			Video:             telegoutil.File(outputFile),
			Thumbnail:         &telego.InputFile{File: thumbnail},
			SupportsStreaming: true,
			Width:             video.Formats.Itag(itag)[0].Width,
			Height:            video.Formats.Itag(itag)[0].Height,
			Caption:           video.Title,
			ReplyParameters: &telego.ReplyParameters{
				MessageID: messageID,
			},
		})
	}
	if err != nil {
		log.Printf("Failed to send video: %v", err)
		return
	}

	defer func() {
		if err := os.Remove(outputFile.Name()); err != nil {
			log.Printf("Failed to remove outputFile: %v", err)
		}
	}()

	bot.DeleteMessage(&telego.DeleteMessageParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
	})
}

func handleYoutubeDownload(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message.GetChat())
	var videoURL string

	if message.ReplyToMessage != nil && message.ReplyToMessage.Text != "" {
		videoURL = message.ReplyToMessage.Text
	} else if len(strings.Fields(message.Text)) > 1 {
		videoURL = strings.Fields(message.Text)[1]
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("medias.youtubeNoURL"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	ytClient := youtube.Client{}
	video, err := ytClient.GetVideo(videoURL)
	if err != nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("medias.youtubeInvalidURL"),
			ParseMode: "HTML",
		})
		return
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

	var audioStream youtube.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[1]
	}

	text := fmt.Sprintf(i18n("medias.youtubeVideoInfo"),
		video.Title, video.Author,
		float64(audioStream.ContentLength)/(1024*1024),
		float64(videoStream.ContentLength+audioStream.ContentLength)/(1024*1024),
		video.Duration.String())

	keyboard := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("medias.youtubeDownloadAudio"),
				CallbackData: fmt.Sprintf("_aud|%s|%d|%d|%d|%d", video.ID, audioStream.ItagNo, audioStream.ContentLength, message.MessageID, message.From.ID),
			},
			telego.InlineKeyboardButton{
				Text:         i18n("medias.youtubeDownloadVideo"),
				CallbackData: fmt.Sprintf("_vid|%s|%d|%d|%d|%d", video.ID, videoStream.ItagNo, videoStream.ContentLength+audioStream.ContentLength, message.MessageID, message.From.ID),
			},
		),
	)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			PreferLargeMedia: true,
		},
		ReplyMarkup: keyboard,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func handleMediaConfig(bot *telego.Bot, update telego.Update) {
	var mediasCaption bool
	var mediasAuto bool
	message := update.Message
	if message == nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	database.DB.QueryRow("SELECT mediasCaption FROM groups WHERE id = ?;", message.Chat.ID).Scan(&mediasCaption)
	database.DB.QueryRow("SELECT mediasAuto FROM groups WHERE id = ?;", message.Chat.ID).Scan(&mediasAuto)

	configType := strings.ReplaceAll(update.CallbackQuery.Data, "mediaConfig ", "")
	if configType != "mediaConfig" {
		query := fmt.Sprintf("UPDATE groups SET %s = ? WHERE id = ?;", configType)
		var err error
		switch configType {
		case "mediasCaption":
			mediasCaption = !mediasCaption
			_, err = database.DB.Exec(query, mediasCaption, message.Chat.ID)
		case "mediasAuto":
			mediasAuto = !mediasAuto
			_, err = database.DB.Exec(query, mediasAuto, message.Chat.ID)
		}
		if err != nil {
			return
		}
	}

	chat := message.GetChat()
	i18n := localization.Get(chat)

	state := func(mediasAuto bool) string {
		if mediasAuto {
			return "✅"
		}
		return "☑️"
	}

	buttons := [][]telego.InlineKeyboardButton{
		{
			{Text: i18n("button.caption"), CallbackData: "ieConfig mediasCaption"},
			{Text: state(mediasCaption), CallbackData: "mediaConfig mediasCaption"},
		},
		{
			{Text: i18n("button.automatic"), CallbackData: "ieConfig mediasAuto"},
			{Text: state(mediasAuto), CallbackData: "mediaConfig mediasAuto"},
		},
	}

	buttons = append(buttons, []telego.InlineKeyboardButton{{
		Text:         i18n("button.back"),
		CallbackData: "configMenu",
	}})

	if update.Message == nil {
		_, err := bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chat.ID),
			MessageID:   update.CallbackQuery.Message.GetMessageID(),
			Text:        i18n("medias.config"),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
		})
		if err != nil {
			log.Print("[medias/mediaConfig] Error edit mediaConfig: ", err)
		}
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(update.Message.Chat.ID),
			Text:        i18n("medias.config"),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
		})
	}
}

func handleExplainConfig(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update.CallbackQuery.Message.(*telego.Message).GetChat())
	ieConfig := strings.ReplaceAll(update.CallbackQuery.Data, "ieConfig medias", "")
	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            i18n("medias." + strings.ToLower(ieConfig) + "Help"),
		ShowAlert:       true,
	})
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("medias")
	bh.HandleMessage(handleYoutubeDownload, telegohandler.CommandEqual("ytdl"))
	bh.HandleMessage(handleMediaDownload, telegohandler.Or(
		telegohandler.CommandEqual("dl"),
		telegohandler.CommandEqual("sdl"),
		telegohandler.TextMatches(regexp.MustCompile(regexMedia)),
	))
	bh.Handle(handleYoutubeDownloadCallback, telegohandler.CallbackDataMatches(regexp.MustCompile(`^(_(vid|aud))`)))
	bh.Handle(handleMediaConfig, telegohandler.CallbackDataPrefix("mediaConfig"), helpers.IsAdmin(bot))
	bh.Handle(handleExplainConfig, telegohandler.CallbackDataPrefix("ieConfig"), helpers.IsAdmin(bot))
}
