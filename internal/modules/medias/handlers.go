package medias

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	var postID string

	if !regexp.MustCompile(`^/(?:s)?dl`).MatchString(message.Text) && strings.Contains(message.Chat.Type, "group") {
		var mediasAuto bool
		if err := database.DB.QueryRow("SELECT mediasAuto FROM groups WHERE id = ?;", message.Chat.ID).Scan(&mediasAuto); err != nil || !mediasAuto {
			return
		}
	}

	url := regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text)
	i18n := localization.Get(message)
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
				postID = result[1]
			}
			break
		}
	}

	row := database.DB.QueryRow("SELECT mediasCaption FROM groups WHERE id = ?;", message.Chat.ID)
	var mediasCaption bool = true
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

	if caption == "" {
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

	replied, err := bot.SendMediaGroup(&telego.SendMediaGroupParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Media:  mediaItems,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	downloader.RemoveMediaFiles(mediaItems)
	if err != nil {
		return
	}
	err = downloader.SetMediaCache(replied, postID)
	if err != nil {
		log.Print(err)
	}
}

func callbackYoutubeDownload(bot *telego.Bot, update telego.Update) {
	message := update.CallbackQuery.Message.(*telego.Message)
	i18n := localization.Get(update)

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
		ChatID:    telegoutil.ID(message.Chat.ID),
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
		ChatID:    telegoutil.ID(message.Chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("medias.uploading"),
	})
	bot.SendChatAction(&telego.SendChatActionParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Action: action,
	})

	_, err = outputFile.Seek(0, 0)
	if err != nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			MessageID: update.CallbackQuery.Message.GetMessageID(),
			Text:      i18n("medias.error"),
		})
		return
	}
	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnail, _ := downloader.Downloader(thumbURL)

	defer func() {
		if err := os.Remove(thumbnail.Name()); err != nil {
			log.Printf("Failed to remove thumbnail: %v", err)
		}
		if err := os.Remove(outputFile.Name()); err != nil {
			log.Printf("Failed to remove outputFile: %v", err)
		}
	}()

	switch callbackData[0] {
	case "_aud":
		_, err = bot.SendAudio(&telego.SendAudioParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Audio:     telegoutil.File(outputFile),
			Thumbnail: &telego.InputFile{File: thumbnail},
			Performer: video.Author,
			Title:     video.Title,
			Caption:   fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: messageID,
			},
		})
	case "_vid":
		_, err = bot.SendVideo(&telego.SendVideoParams{
			ChatID:            telegoutil.ID(message.Chat.ID),
			Video:             telegoutil.File(outputFile),
			Thumbnail:         &telego.InputFile{File: thumbnail},
			SupportsStreaming: true,
			Width:             video.Formats.Itag(itag)[0].Width,
			Height:            video.Formats.Itag(itag)[0].Height,
			Caption:           fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			ParseMode:         "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: messageID,
			},
		})
	}
	if err != nil {
		log.Printf("Failed to send video: %v", err)
		return
	}

	bot.DeleteMessage(&telego.DeleteMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
	})
}

func handleYoutubeDownload(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message)
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
	var client *http.Client
	if config.Socks5Proxy != "" {
		proxyURL, _ := url.Parse(config.Socks5Proxy)
		client = &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}}
		ytClient = youtube.Client{HTTPClient: client}
	}
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

func handleExplainConfig(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)
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
	bh.Handle(callbackYoutubeDownload, telegohandler.CallbackDataMatches(regexp.MustCompile(`^(_(vid|aud))`)))
	bh.Handle(handleExplainConfig, telegohandler.CallbackDataPrefix("ieConfig"), helpers.IsAdmin(bot))
}
