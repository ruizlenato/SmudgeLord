package modules

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"
	"smudgelord/smudgelord/utils/medias"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

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
	url := regexp.MustCompile(`(?:htt.*?//).+(?:instagram|twitter|x|tiktok|reddit|twitch)\.(?:com|net|tv)\/(?:\S*)`).FindStringSubmatch(message.Text)
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
	}

	bot.SendMediaGroup(&telego.SendMediaGroupParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Media:  mediaItems,
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
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
			log.Print("Error edit mediaConfig: ", err)
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

func LoadMediaDownloader(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("medias")
	bh.HandleMessage(mediaDownloader, telegohandler.CommandEqual("dl"))
	bh.HandleMessage(mediaDownloader, telegohandler.CommandEqual("sdl"))
	bh.HandleMessage(mediaDownloader, telegohandler.TextMatches(regexp.MustCompile(`(?:htt.*?//).+(?:instagram|twitter|x|tiktok|reddit|twitch)\.(?:com|net|tv)\/(?:\S*)`)))
	bh.Handle(mediaConfig, telegohandler.CallbackDataPrefix("mediaConfig"), helpers.IsAdmin(bot))
	bh.Handle(explainConfig, telegohandler.CallbackDataPrefix("ieConfig"), helpers.IsAdmin(bot))
}
