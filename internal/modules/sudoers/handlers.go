package sudoers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var announceMessageText string

func announceHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	var lang string
	message := update.Message

	if message == nil {
		message = update.CallbackQuery.Message.Message
		lang = strings.ReplaceAll(update.CallbackQuery.Data, "announce ", "")
	}

	if (message == nil || message.From.ID != config.OwnerID) &&
		(update.CallbackQuery == nil || update.CallbackQuery.From.ID != config.OwnerID) {
		return
	}

	if lang == "" {
		buttons := make([][]models.InlineKeyboardButton, 0, len(database.AvailableLocales))
		for _, lang := range database.AvailableLocales {
			loaded, ok := localization.LangBundles[lang]
			if !ok {
				slog.Error("Language not found in the cache",
					"lang", lang)
				os.Exit(1)

			}
			languageFlag, _, _ := loaded.FormatMessage("language-flag")
			languageName, _, _ := loaded.FormatMessage("language-name")

			buttons = append(buttons, []models.InlineKeyboardButton{{
				Text: languageFlag +
					languageName,
				CallbackData: fmt.Sprintf("announce %s", lang),
			}})
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "Choose a language:",
			ParseMode: models.ParseModeHTML,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: buttons,
			},
		})
		announceMessageText = utils.FormatText(message.Text, message.Entities)
		return
	}

	messageFields := strings.Fields(announceMessageText)
	if len(messageFields) < 2 {
		return
	}

	announceType := messageFields[1]
	announceMessageText = strings.Replace(announceMessageText, messageFields[0], "", 1)
	var query string

	switch announceType {
	case "groups":
		announceMessageText = strings.Replace(announceMessageText, announceType, "", 1)
		query = fmt.Sprintf("SELECT id FROM groups WHERE language = '%s';", lang)
	case "users":
		announceMessageText = strings.Replace(announceMessageText, announceType, "", 1)
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s';", lang)
	default:
		query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s' UNION ALL SELECT id FROM groups WHERE language = '%s';", lang, lang)
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
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      announceMessageText,
			ParseMode: models.ParseModeHTML,
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

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      fmt.Sprintf("<b>Messages sent successfully:</b> <code>%d</code>\n<b>Messages unsent:</b> <code>%d</code>", successCount, errorCount),
		ParseMode: models.ParseModeHTML,
	})
	announceMessageText = ""
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "announce", bot.MatchTypeCommand, announceHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "announce", bot.MatchTypePrefix, announceHandler)
}
