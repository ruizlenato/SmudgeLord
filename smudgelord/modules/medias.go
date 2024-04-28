package modules

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"
	"smudgelord/smudgelord/utils/medias"

	"github.com/kkdai/youtube/v2"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

const regexMedia = `(?:http(?:s)?://)?(?:m|vm|www|mobile)?(?:.)?(?:instagram|twitter|x|tiktok|reddit|twitch).(?:com|net|tv)/(?:\S*)`

func removeMediaFiles(mediaItems []telego.InputMedia) {
	var wg sync.WaitGroup

	for _, media := range mediaItems {
		wg.Add(1)

		go func(media telego.InputMedia) {
			defer wg.Done()

			switch media.MediaType() {
			case "photo":
				if photo, ok := media.(*telego.InputMediaPhoto); ok {
					os.Remove(photo.Media.String())
				}
			case "video":
				if video, ok := media.(*telego.InputMediaVideo); ok {
					os.Remove(video.Media.String())
				}
			}
		}(media)
	}

	wg.Wait()
}

func mediaDownloader(bot *telego.Bot, message telego.Message) {
	if !regexp.MustCompile(`^/(?:s)?dl`).MatchString(message.Text) && strings.Contains(message.Chat.Type, "group") {
		row := database.DB.QueryRow("SELECT mediasAuto FROM groups WHERE id = ?;", message.Chat.ID)
		var mediasAuto bool
		if row.Scan(&mediasAuto); !mediasAuto {
			return
		}
	}

	i18n := localization.Get(message.GetChat())

	// Extract URL from the message text using regex
	url := regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text)
	if len(url) < 1 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("medias.noURL"),
			ParseMode: "HTML",
		})
		return
	}

	dm := medias.NewDownloadMedia()
	mediaItems, caption := dm.Download(url[0])
	if strings.Contains(message.Chat.Type, "group") {
		row := database.DB.QueryRow("SELECT mediasCaption FROM groups WHERE id = ?;", message.Chat.ID)
		var mediasCaption bool
		if row.Scan(&mediasCaption); !mediasCaption {
			caption = fmt.Sprintf("<a href='%s'>🔗 Link</a>", url[0])
		}
	}

	// Check if only one photo is present and link preview is enabled, then return
	if len(mediaItems) == 1 && mediaItems[0].MediaType() == "photo" && !message.LinkPreviewOptions.IsDisabled {
		return
	}

	if len(mediaItems) > 0 {
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
		removeMediaFiles(mediaItems)
	}
}

func mediaConfig(bot *telego.Bot, update telego.Update) {
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

	// Verificar porque o "update.CallbackQuery.Message.GetMessageID()" não atualiza após ser chamado novamente

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

func explainConfig(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update.CallbackQuery.Message.(*telego.Message).GetChat())
	ieConfig := strings.ReplaceAll(update.CallbackQuery.Data, "ieConfig medias", "")
	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            i18n("medias." + strings.ToLower(ieConfig) + "Help"),
		ShowAlert:       true,
	})
}

func cliYTDL(bot *telego.Bot, update telego.Update) {
	callbackData := strings.Split(update.CallbackQuery.Data, "|")
	itag, _ := strconv.Atoi(callbackData[2])

	var err error
	var outputFile *os.File
	var action string

	client := youtube.Client{}
	video, err := client.GetVideo(callbackData[1])
	if err != nil {
		return
	}
	format := video.Formats.Itag(itag)[0]

	// Create temporary audio/video file and set the chat action.
	switch callbackData[0] {
	case "_aud":
		outputFile, err = os.CreateTemp("", "youtubeSmudge_*.mp3")
		action = telego.ChatActionUploadVoice
	case "_vid":
		outputFile, err = os.CreateTemp("", "youtubeSmudge_*.mp4")
		action = telego.ChatActionUploadVideo
	}
	if err != nil {
		log.Println("[medias/cliYTDL] Error creating temporary file: ", err)
		return
	}

	stream, _, err := client.GetStream(video, &format)
	if err != nil {
		log.Println("[medias/cliYTDL] Error getting stream: ", err)
		return
	}

	_, err = io.Copy(outputFile, stream)
	stream.Close()
	if err != nil {
		log.Println("[medias/cliYTDL] Error seeking outputFile: ", err)
		return
	}
	if callbackData[0] == "_vid" {
		stream, _, err := client.GetStream(video, &video.Formats.Itag(140)[0])
		if err != nil {
			log.Println("[medias/cliYTDL] Error getting audio stream: ", err)
			return
		}
		audioFile, err := os.CreateTemp("", "youtube_*.m4a")
		if err != nil {
			log.Println("[medias/cliYTDL] Error creating temporary audioFile: ", err)
			return
		}
		_, err = io.Copy(audioFile, stream)
		stream.Close()
		if err != nil {
			log.Println("[medias/cliYTDL] Error seeking audioFile: ", err)
			return
		}

		outputFile = medias.MergeAudioVideo(outputFile, audioFile)
	}

	chatID := update.CallbackQuery.Message.GetChat().ID
	bot.DeleteMessage(&telego.DeleteMessageParams{
		ChatID:    telegoutil.ID(chatID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
	})

	bot.SendChatAction(&telego.SendChatActionParams{
		ChatID: telegoutil.ID(chatID),
		Action: action,
	})

	outputFile.Seek(0, 0) // Seek back to the beginning of the file
	switch callbackData[0] {
	case "_aud":
		bot.SendAudio(&telego.SendAudioParams{
			ChatID: telegoutil.ID(chatID),
			Audio:  telegoutil.File(outputFile),
			Title:  video.Title,
		})
	case "_vid":
		bot.SendVideo(&telego.SendVideoParams{
			ChatID:  telegoutil.ID(chatID),
			Video:   telegoutil.File(outputFile),
			Width:   format.Width,
			Height:  format.Height,
			Caption: video.Title,
		})
	}
	// Remove the temporary file
	os.Remove(outputFile.Name())
}

func youtubeDL(bot *telego.Bot, message telego.Message) {
	if len(strings.Fields(message.Text)) < 2 {
		return
	}
	videoURL := strings.Fields(message.Text)[1]
	ytClient := youtube.Client{}
	video, err := ytClient.GetVideo(videoURL)
	if err != nil {
		log.Println("[medias/youtubeDL] Error getting video: ", err)
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
	videoSize := videoStream.ContentLength

	var audioStream youtube.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[1]
	}
	audioSize := audioStream.ContentLength

	text := fmt.Sprintf("📹 <b>%s</b> - <i>%s</i>", video.Author, video.Title)
	text += fmt.Sprintf("\n💾 <code>%.2f MB</code> (audio) | <code>%.2f MB</code> (video)", float64(audioSize)/(1024*1024), float64(audioSize)/(1024*1024)+float64(videoSize)/(1024*1024))
	text += fmt.Sprintf("\n⏳ <code>%s</code>", video.Duration.String())

	keyboard := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         "💿 Áudio",
				CallbackData: fmt.Sprintf("_aud|%s|%d", video.ID, audioStream.ItagNo),
			},
			telego.InlineKeyboardButton{
				Text:         "🎬 Vídeo",
				CallbackData: fmt.Sprintf("_vid|%s|%d", video.ID, videoStream.ItagNo),
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

func LoadMediaDownloader(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("medias")
	bh.HandleMessage(youtubeDL, telegohandler.CommandEqual("ytdl"))
	bh.HandleMessage(mediaDownloader, telegohandler.Or(
		telegohandler.CommandEqual("dl"),
		telegohandler.CommandEqual("sdl"),
		telegohandler.TextMatches(regexp.MustCompile(regexMedia)),
	))
	bh.Handle(cliYTDL, telegohandler.CallbackDataMatches(regexp.MustCompile(`^(_(vid|aud))`)))
	bh.Handle(mediaConfig, telegohandler.CallbackDataPrefix("mediaConfig"), helpers.IsAdmin(bot))
	bh.Handle(explainConfig, telegohandler.CallbackDataPrefix("ieConfig"), helpers.IsAdmin(bot))
}
