package sudoers

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
)

var announceMessageText string

func announceHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	var lang string
	message := ctx.EffectiveMessage

	if ctx.CallbackQuery != nil {
		lang = strings.ReplaceAll(ctx.CallbackQuery.Data, "announce ", "")
	}

	if (ctx.EffectiveUser == nil || ctx.EffectiveUser.Id != config.OwnerID) &&
		(ctx.CallbackQuery == nil || ctx.CallbackQuery.From.Id != config.OwnerID) {
		return nil
	}

	if lang == "" {
		if message == nil {
			return nil
		}

		buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(database.AvailableLocales))
		for _, locale := range database.AvailableLocales {
			loaded, ok := localization.LangBundles[locale]
			if !ok {
				slog.Error("Language not found in the cache", "lang", locale)
				os.Exit(1)
			}

			languageFlag, _, _ := loaded.FormatMessage("language-flag")
			languageName, _, _ := loaded.FormatMessage("language-name")

			buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
				Text:         languageFlag + languageName,
				CallbackData: fmt.Sprintf("announce %s", locale),
			}})
		}

		_, _ = b.SendMessage(message.Chat.Id, "Choose a language:", &gotgbot.SendMessageOpts{
			ParseMode: gotgbot.ParseModeHTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: buttons,
			},
		})

		announceMessageText = message.GetText()
		return nil
	}

	messageFields := strings.Fields(announceMessageText)
	if len(messageFields) < 2 {
		return nil
	}

	announceType := messageFields[1]
	announceMessageText = strings.Replace(announceMessageText, messageFields[0], "", 1)
	var query string

	switch announceType {
	case "groups":
		announceMessageText = strings.Replace(announceMessageText, announceType, "", 1)
		query = fmt.Sprintf("SELECT id FROM chats WHERE language = '%s';", lang)
	case "users":
		announceMessageText = strings.Replace(announceMessageText, announceType, "", 1)
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s';", lang)
	default:
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s' UNION ALL SELECT id FROM chats WHERE language = '%s';", lang, lang)
	}

	rows, err := database.DB.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var successCount, errorCount int
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			continue
		}

		_, err := b.SendMessage(chatID, announceMessageText, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		if err != nil {
			errorCount++
			continue
		}
		successCount++
	}

	if ctx.CallbackQuery != nil && ctx.CallbackQuery.Message != nil {
		chat := ctx.CallbackQuery.Message.GetChat()
		msgID := ctx.CallbackQuery.Message.GetMessageId()
		_, _, _ = b.EditMessageText(
			fmt.Sprintf("<b>Messages sent successfully:</b> <code>%d</code>\n<b>Messages unsent:</b> <code>%d</code>", successCount, errorCount),
			&gotgbot.EditMessageTextOpts{
				ChatId:    chat.Id,
				MessageId: msgID,
				ParseMode: gotgbot.ParseModeHTML,
			},
		)
	}

	announceMessageText = ""
	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("announce", announceHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("announce"), announceHandler))
}
