package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func disableableHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	text := i18n("disableables-commands")

	for _, command := range utils.DisableableCommands {
		text += "\n- <code>" + command + "</code>"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			PreferLargeMedia: bot.True(),
			ShowAboveText:    bot.True(),
		},
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func disableHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	contains := func(array []string, str string) bool {
		for _, item := range array {
			if item == str {
				return true
			}
		}
		return false
	}

	if len(strings.Fields(update.Message.Text)) > 1 {
		command := strings.Fields(update.Message.Text)[1]
		if !contains(utils.DisableableCommands, command) {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: i18n("command-not-deactivatable",
					map[string]interface{}{
						"command": command,
					}),
				ParseMode: "HTML",
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
			return
		}

		if utils.CheckDisabledCommand(command, update.Message.Chat.ID) {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: i18n("command-already-disabled",
					map[string]interface{}{
						"command": command,
					}),
				ParseMode: "HTML",
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
			return
		}

		if err := insertDisabledCommand(update.Message.Chat.ID, command); err != nil {
			fmt.Print("Error inserting command: " + err.Error())
			return
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: i18n("command-disabled",
				map[string]interface{}{
					"command": command,
				}),
			ParseMode: "HTML",
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("disable-commands-usage"),
		ParseMode: "HTML",
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func enableHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	if len(strings.Fields(update.Message.Text)) > 1 {
		command := strings.Fields(update.Message.Text)[1]

		if !utils.CheckDisabledCommand(command, update.Message.Chat.ID) {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: i18n("command-already-enabled",
					map[string]interface{}{
						"command": command,
					}),
				ParseMode: "HTML",
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
			return
		}

		if err := deleteDisabledCommand(command); err != nil {
			fmt.Print("Error deleting command: " + err.Error())
			return
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: i18n("command-enabled",
				map[string]interface{}{
					"command": command,
				}),
			ParseMode: "HTML",
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("enable-commands-usage"),
		ParseMode: "HTML",
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func disabledHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	text := i18n("disabled-commands")
	commands, err := getDisabledCommands(update.Message.Chat.ID)
	if err != nil {
		return
	}
	if len(commands) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("no-disabled-commands"),
			ParseMode: "HTML",
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	for _, command := range commands {
		text += "\n- <code>" + command + "</code>"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: "HTML",
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func languageMenuCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	buttons := make([][]models.InlineKeyboardButton, 0, len(database.AvailableLocales))
	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangBundles[lang]
		if !ok {
			slog.Error("Language not found in the cache",
				"lang", lang,
				"availableLocales", database.AvailableLocales)
			os.Exit(1)

		}
		languageFlag, _, _ := loaded.FormatMessage("language-flag")
		languageName, _, _ := loaded.FormatMessage("language-name")

		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text:         languageFlag + languageName,
			CallbackData: fmt.Sprintf("setLang %s", lang),
		}})
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text: i18n("language-menu",
			map[string]any{
				"languageFlag": i18n("language-flag"),
				"languageName": i18n("language-name"),
			}),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
}

func setLanguageCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	lang := strings.ReplaceAll(update.CallbackQuery.Data, "setLang ", "")

	dbQuery := "UPDATE groups SET language = ? WHERE id = ?;"
	if update.CallbackQuery.Message.Message.Chat.Type == models.ChatTypePrivate {
		dbQuery = "UPDATE users SET language = ? WHERE id = ?;"
	}
	_, err := database.DB.Exec(dbQuery, lang, update.CallbackQuery.Message.Message.Chat.ID)
	if err != nil {
		slog.Error("Couldn't update language",
			"ChatID", update.CallbackQuery.Message.Message.ID,
			"Error", err.Error())
	}

	buttons := make([][]models.InlineKeyboardButton, 0, len(database.AvailableLocales))

	if update.CallbackQuery.Message.Message.Chat.Type == models.ChatTypePrivate {
		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text:         i18n("back-button"),
			CallbackData: "start",
		}})
	} else {
		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text:         i18n("back-button"),
			CallbackData: "config",
		}})
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("language-changed"),

		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
}

func createConfigKeyboard(i18n func(string, ...map[string]any) string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         i18n("medias"),
					CallbackData: "mediaConfig",
				},
			},
			{
				{
					Text:         i18n("language-flag") + i18n("language-button"),
					CallbackData: "languageMenu",
				},
			},
		},
	}
}

func configHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("config-message"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: createConfigKeyboard(i18n),
	})
}

func configCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        i18n("config-message"),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: createConfigKeyboard(i18n),
	})
}

func getMediaConfig(chatID int64) (bool, bool, error) {
	var mediasCaption, mediasAuto bool
	err := database.DB.QueryRow("SELECT mediasCaption, mediasAuto FROM groups WHERE id = ?;", chatID).Scan(&mediasCaption, &mediasAuto)
	return mediasCaption, mediasAuto, err
}

func mediaConfigCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	mediasCaption, mediasAuto, err := getMediaConfig(update.CallbackQuery.Message.Message.Chat.ID)
	if err != nil {
		slog.Error("Couldn't query media config",
			"ChatID", update.CallbackQuery.Message.Message.Chat.ID,
			"Error", err.Error())
		return
	}

	configType := strings.ReplaceAll(update.CallbackQuery.Data, "mediaConfig ", "")
	if configType != "mediaConfig" {
		query := fmt.Sprintf("UPDATE groups SET %s = ? WHERE id = ?;", configType)
		var err error
		switch configType {
		case "mediasCaption":
			mediasCaption = !mediasCaption
			_, err = database.DB.Exec(query, mediasCaption, update.CallbackQuery.Message.Message.Chat.ID)
		case "mediasAuto":
			mediasAuto = !mediasAuto
			_, err = database.DB.Exec(query, mediasAuto, update.CallbackQuery.Message.Message.Chat.ID)
		}
		if err != nil {
			return
		}
	}
	i18n := localization.Get(update)

	state := func(mediasAuto bool) string {
		if mediasAuto {
			return "✅"
		}
		return "☑️"
	}

	buttons := [][]models.InlineKeyboardButton{
		{
			{
				Text:         i18n("caption-button"),
				CallbackData: "ieConfig mediasCaption",
			},
			{
				Text:         state(mediasCaption),
				CallbackData: "mediaConfig mediasCaption",
			},
		},
		{
			{
				Text:         i18n("automatic-button"),
				CallbackData: "ieConfig mediasAuto",
			},
			{
				Text:         state(mediasAuto),
				CallbackData: "mediaConfig mediasAuto",
			},
		},
	}

	buttons = append(buttons, []models.InlineKeyboardButton{{
		Text:         i18n("back-button"),
		CallbackData: "config",
	}})

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        i18n("config-medias"),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
}

func explainConfigCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	ieConfig := strings.ReplaceAll(update.CallbackQuery.Data, "ieConfig medias", "")
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            i18n("ieConfig-" + ieConfig),
		ShowAlert:       true,
	})
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "languageMenu", bot.MatchTypeExact, languageMenuCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "setLang", bot.MatchTypeContains, setLanguageCallback)
	b.RegisterHandler(bot.HandlerTypeMessageText, "config", bot.MatchTypeCommand, configHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "config", bot.MatchTypeExact, configCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "mediaConfig", bot.MatchTypeContains, mediaConfigCallback)
	b.RegisterHandler(bot.HandlerTypeMessageText, "disableable", bot.MatchTypeCommand, disableableHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "disable", bot.MatchTypeCommand, disableHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "enable", bot.MatchTypeCommand, enableHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "disabled", bot.MatchTypeCommand, disabledHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "disableable", bot.MatchTypeCommand, disableableHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "ieConfig", bot.MatchTypeExact, explainConfigCallback)

	utils.SaveHelp("config")
}
