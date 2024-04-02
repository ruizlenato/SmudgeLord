package modules

import (
	"fmt"
	"log"
	"strings"

	"smudgelord/smudgelord/config"
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

var announceMessageText string // Variável global para armazenar o texto da mensagem

func announce(bot *telego.Bot, update telego.Update) {
	var lang string
	message := update.Message

	if message == nil {
		message = update.CallbackQuery.Message.(*telego.Message)
		lang = strings.ReplaceAll(update.CallbackQuery.Data, "announce ", "")
	}

	if (message == nil || message.From.ID != config.OWNER_ID) &&
		(update.CallbackQuery == nil || update.CallbackQuery.From.ID != config.OWNER_ID) {
		return
	}

	if lang == "" {
		buttons := make([][]telego.InlineKeyboardButton, 0, len(database.AvailableLocales))
		for _, lang := range database.AvailableLocales {
			loaded, ok := localization.LangCache[lang]
			if !ok {
				log.Fatalf("Language '%s' not found in the cache.", lang)
			}

			buttons = append(buttons, []telego.InlineKeyboardButton{{
				Text: localization.GetStringFromNestedMap(loaded, "language.flag") +
					localization.GetStringFromNestedMap(loaded, "language.name"),
				CallbackData: fmt.Sprintf("announce %s", lang),
			}})
		}

		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(config.OWNER_ID),
			Text:        "Choose a language:",
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
		})
		announceMessageText = message.Text
		return
	}

	messageFields := strings.Fields(announceMessageText)
	if len(messageFields) < 2 {
		return
	}

	announceType := messageFields[1]
	announceText := strings.Join(messageFields[2:], " ")
	var query string

	switch announceType {
	case "groups":
		query = fmt.Sprintf("SELECT id FROM groups WHERE language = '%s';", lang)
	case "users":
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s';", lang)
	default:
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s' UNION ALL SELECT id FROM groups WHERE language = '%s';", lang, lang)
		announceText = strings.Join(messageFields[1:], " ")
		if len(messageFields) > 2 {
			announceText = strings.Join(messageFields[2:], " ")
		}
	}

	rows, err := database.DB.Query(query)
	if err != nil {
		return
	}
	defer rows.Close()

	var successCount, errorCount int

	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			continue
		}

		_, err := bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(chatID),
			Text:      announceText,
			ParseMode: "HTML",
		})
		if err != nil {
			errorCount++
			continue
		}

		successCount++
	}

	if err := rows.Err(); err != nil {
		return
	}

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(config.OWNER_ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      fmt.Sprintf("<b>Messages sent successfully:</b> <code>%d</code>\n<b>Messages unsent:</b> <code>%d</code>", successCount, errorCount),
		ParseMode: "HTML",
	})
	announceMessageText = ""
}

func LoadSudoers(bh *telegohandler.BotHandler, bot *telego.Bot) {
	bh.Handle(announce, telegohandler.CommandEqual("announce"))
	bh.Handle(announce, telegohandler.CallbackDataPrefix("announce"))
}
