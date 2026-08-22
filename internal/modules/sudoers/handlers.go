// Package sudoers manages bot administrator users and permissions.
package sudoers

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
)

var announceState struct {
	mu            sync.Mutex
	messageText   string
	lang          string
	broadcastType string
}

func announceSetMessage(text string) {
	announceState.mu.Lock()
	defer announceState.mu.Unlock()
	announceState.messageText = text
	announceState.lang = ""
	announceState.broadcastType = ""
}

func announceSetLang(lang string) {
	announceState.mu.Lock()
	defer announceState.mu.Unlock()
	announceState.lang = lang
	announceState.broadcastType = ""
}

func announceSetType(broadcastType string) {
	announceState.mu.Lock()
	defer announceState.mu.Unlock()
	announceState.broadcastType = broadcastType
}

func announceReset() {
	announceState.mu.Lock()
	defer announceState.mu.Unlock()
	announceState.messageText = ""
	announceState.lang = ""
	announceState.broadcastType = ""
}

func announceSnapshot() (messageText, lang, broadcastType string) {
	announceState.mu.Lock()
	defer announceState.mu.Unlock()
	return announceState.messageText, announceState.lang, announceState.broadcastType
}

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

		announceSetMessage(messageText)

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
		announceSetLang(strings.TrimPrefix(callbackData, "announce:lang:"))

		chat := ctx.CallbackQuery.Message.GetChat()
		msgID := ctx.CallbackQuery.Message.GetMessageId()
		_, _, _ = b.EditMessageText(&gotgbot.EditMessageTextOpts{
			Text:      "Choose broadcast type:",
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
		announceSetType(strings.TrimPrefix(callbackData, "announce:type:"))

		chat := ctx.CallbackQuery.Message.GetChat()
		msgID := ctx.CallbackQuery.Message.GetMessageId()
		_, _, _ = b.EditMessageText(&gotgbot.EditMessageTextOpts{
			Text:      "Do you really want to send this message?",
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
		announceReset()

		if ctx.CallbackQuery != nil && ctx.CallbackQuery.Message != nil {
			chat := ctx.CallbackQuery.Message.GetChat()
			msgID := ctx.CallbackQuery.Message.GetMessageId()
			_, _, _ = b.EditMessageText(&gotgbot.EditMessageTextOpts{
				Text:      "Broadcast canceled.",
				ChatId:    chat.Id,
				MessageId: msgID,
				ParseMode: gotgbot.ParseModeHTML,
			})
		}
		return nil
	}

	messageText, lang, broadcastType := announceSnapshot()
	if callbackData != "announce:confirm:yes" || messageText == "" || lang == "" || broadcastType == "" {
		return nil
	}

	var query string
	var args []any
	switch broadcastType {
	case "groups":
		query = "SELECT id FROM chats WHERE language = ?;"
		args = []any{lang}
	case "users":
		query = "SELECT id FROM users WHERE language = ?;"
		args = []any{lang}
	default:
		query = "SELECT id FROM users WHERE language = ? UNION ALL SELECT id FROM chats WHERE language = ?;"
		args = []any{lang, lang}
	}

	rows, err := database.DB.Query(query, args...)
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

		_, err := b.SendMessage(chatID, messageText, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		if err != nil {
			errorCount++
			continue
		}
		successCount++
	}

	if ctx.CallbackQuery != nil && ctx.CallbackQuery.Message != nil {
		chat := ctx.CallbackQuery.Message.GetChat()
		msgID := ctx.CallbackQuery.Message.GetMessageId()
		_, _, _ = b.EditMessageText(&gotgbot.EditMessageTextOpts{
			Text:      fmt.Sprintf("<b>Messages sent successfully:</b> <code>%d</code>\n<b>Messages unsent:</b> <code>%d</code>", successCount, errorCount),
			ChatId:    chat.Id,
			MessageId: msgID,
			ParseMode: gotgbot.ParseModeHTML,
		})
	}

	announceReset()
	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("announce", announceHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("announce"), announceHandler))
}
