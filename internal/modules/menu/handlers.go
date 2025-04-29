package menu

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func createStartKeyboard(i18n func(string, ...map[string]any) string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         i18n("about-button"),
					CallbackData: "about",
				},
				{
					Text:         fmt.Sprintf("%s %s", i18n("language-flag"), i18n("language-button")),
					CallbackData: "languageMenu",
				},
			},
			{
				{
					Text:         i18n("help-button"),
					CallbackData: "helpMenu",
				},
			},
		},
	}
}

func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	botUser, err := b.GetMe(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if messageFields := strings.Fields(update.Message.Text); len(messageFields) > 1 && messageFields[1] == "privacy" {
		privacyHandler(ctx, b, update)
		return
	}

	if update.Message.Chat.Type == models.ChatTypeGroup || update.Message.Chat.Type == models.ChatTypeSupergroup {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: i18n("start-message-group",
				map[string]any{
					"botName": botUser.FirstName,
				}),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{{
					Text: i18n("start-button"),
					URL:  fmt.Sprintf("https://t.me/%s?start=start", botUser.Username),
				}}},
			},
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: i18n("start-message",
			map[string]any{
				"userFirstName": update.Message.From.FirstName,
				"botName":       botUser.FirstName,
			}),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: createStartKeyboard(i18n),
	})
}

func startCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	botUser, err := b.GetMe(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text: i18n("start-message",
			map[string]any{
				"userFirstName": update.CallbackQuery.Message.Message.From.FirstName,
				"botName":       botUser.FirstName,
			}),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: createStartKeyboard(i18n),
	})
}

func privacyHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	botUser, err := b.GetMe(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if update.Message.Chat.Type == models.ChatTypeGroup || update.Message.Chat.Type == models.ChatTypeSupergroup {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("privacy-policy-group"),
			ParseMode: "HTML",
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{{
					Text: i18n("privacy-policy-button"),
					URL:  fmt.Sprintf("https://t.me/%s?start=privacy", botUser.Username),
				}}},
			},
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("privacy-policy-private"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{
				Text:         i18n("about-your-data-button"),
				CallbackData: "aboutYourData",
			}}},
		},
	})
}

func privacyCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("privacy-policy-private"),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:         i18n("about-your-data-button"),
						CallbackData: "aboutYourData",
					},
				},
				{
					{
						Text:         i18n("back-button"),
						CallbackData: "about",
					},
				},
			},
		},
	})
}

func aboutMenuCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("about"),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text: i18n("donation-button"),
						URL:  "https://ko-fi.com/ruizlenato",
					},
					{
						Text: i18n("news-channel-button"),
						URL:  "https://t.me/SmudgeLordChannel",
					},
				},
				{
					{
						Text:         i18n("privacy-policy-button"),
						CallbackData: "privacy",
					},
				},
				{
					{
						Text:         i18n("back-button"),
						CallbackData: "start",
					},
				},
			},
		},
	})
}

func aboutYourDataCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("about-your-data"),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:         i18n("back-button"),
						CallbackData: "privacy",
					},
				},
			},
		},
	})
}

func helpMenuCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("help"),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: utils.GetHelpKeyboard(i18n),
		},
	})
}

func helpMessageCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	module := strings.ReplaceAll(update.CallbackQuery.Data, "helpMessage ", "")

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n(fmt.Sprintf("%s-help", module)),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:         i18n("back-button"),
						CallbackData: "helpMenu",
					},
				},
			},
		},
	})
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommand, startHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "start", bot.MatchTypeExact, startCallback)
	b.RegisterHandler(bot.HandlerTypeMessageText, "privacy", bot.MatchTypeCommand, privacyHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "privacy", bot.MatchTypeExact, privacyCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "about", bot.MatchTypeExact, aboutMenuCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "aboutYourData", bot.MatchTypeExact, aboutYourDataCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "helpMenu", bot.MatchTypeExact, helpMenuCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "helpMessage", bot.MatchTypePrefix, helpMessageCallback)
}
