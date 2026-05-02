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
var announceLang string
var announceType string

func announceHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	message := ctx.EffectiveMessage
	callbackData := ""

	if ctx.CallbackQuery != nil {
		callbackData = ctx.CallbackQuery.Data
	}

	if (ctx.EffectiveUser == nil || ctx.EffectiveUser.Id != config.OwnerID) &&
		(ctx.CallbackQuery == nil || ctx.CallbackQuery.From.Id != config.OwnerID) {
		return nil
	}

	if callbackData == "" {
		if message == nil {
			return nil
		}

		messageText := strings.TrimSpace(strings.TrimPrefix(message.GetText(), "/announce"))
		if messageText == "" {
			_, _ = b.SendMessage(message.Chat.Id, "Use the command as: <code>/announce your message</code>", &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
			return nil
		}

		announceMessageText = messageText
		announceLang = ""
		announceType = ""

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
				CallbackData: fmt.Sprintf("announce:lang:%s", locale),
			}})
		}

		_, _ = b.SendMessage(message.Chat.Id, "Choose a language:", &gotgbot.SendMessageOpts{
			ParseMode: gotgbot.ParseModeHTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: buttons,
			},
		})

		return nil
	}

	if strings.HasPrefix(callbackData, "announce:lang:") {
		announceLang = strings.TrimPrefix(callbackData, "announce:lang:")
		announceType = ""

		chat := ctx.CallbackQuery.Message.GetChat()
		msgID := ctx.CallbackQuery.Message.GetMessageId()
		_, _, _ = b.EditMessageText("Choose broadcast type:", &gotgbot.EditMessageTextOpts{
			ChatId:    chat.Id,
			MessageId: msgID,
			ParseMode: gotgbot.ParseModeHTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{{Text: "Users", CallbackData: "announce:type:users"}},
				{{Text: "Groups", CallbackData: "announce:type:groups"}},
				{{Text: "All", CallbackData: "announce:type:all"}},
			}},
		})
		return nil
	}

	if strings.HasPrefix(callbackData, "announce:type:") {
		announceType = strings.TrimPrefix(callbackData, "announce:type:")

		chat := ctx.CallbackQuery.Message.GetChat()
		msgID := ctx.CallbackQuery.Message.GetMessageId()
		_, _, _ = b.EditMessageText("Do you really want to send this message?", &gotgbot.EditMessageTextOpts{
			ChatId:    chat.Id,
			MessageId: msgID,
			ParseMode: gotgbot.ParseModeHTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{{Text: "Yes", CallbackData: "announce:confirm:yes", Style: gotgbot.KeyboardButtonStyleSuccess}},
				{{Text: "No", CallbackData: "announce:confirm:no", Style: gotgbot.KeyboardButtonStyleDanger}},
			}},
		})
		return nil
	}

	if callbackData == "announce:confirm:no" {
		announceMessageText = ""
		announceLang = ""
		announceType = ""

		if ctx.CallbackQuery != nil && ctx.CallbackQuery.Message != nil {
			chat := ctx.CallbackQuery.Message.GetChat()
			msgID := ctx.CallbackQuery.Message.GetMessageId()
			_, _, _ = b.EditMessageText("Broadcast canceled.", &gotgbot.EditMessageTextOpts{
				ChatId:    chat.Id,
				MessageId: msgID,
				ParseMode: gotgbot.ParseModeHTML,
			})
		}
		return nil
	}

	if callbackData != "announce:confirm:yes" || announceMessageText == "" || announceLang == "" || announceType == "" {
		return nil
	}

	var query string
	switch announceType {
	case "groups":
		query = fmt.Sprintf("SELECT id FROM chats WHERE language = '%s';", announceLang)
	case "users":
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s';", announceLang)
	default:
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s' UNION ALL SELECT id FROM chats WHERE language = '%s';", announceLang, announceLang)
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
	announceLang = ""
	announceType = ""
	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("announce", announceHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("announce"), announceHandler))
}
