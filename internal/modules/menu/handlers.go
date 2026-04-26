package menu

import (
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/choseninlineresult"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/lastfm"
	"github.com/ruizlenato/smudgelord/internal/modules/medias"
	"github.com/ruizlenato/smudgelord/internal/modules/misc"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func createStartKeyboard(i18n func(string, ...map[string]any) string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
		{{Text: i18n("about-button"), CallbackData: "about"}, {Text: fmt.Sprintf("%s %s", i18n("language-flag"), i18n("language-button")), CallbackData: "languageMenu"}},
		{{Text: i18n("help-button"), CallbackData: "helpMenu"}},
	}}
}

func startHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	botUser, err := b.GetMe(nil)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if messageFields := strings.Fields(ctx.EffectiveMessage.GetText()); len(messageFields) > 1 {
		switch messageFields[1] {
		case "privacy":
			return privacyHandler(b, ctx)
		case "setuser":
			return lastfm.StartSetUser(b, ctx)
		}
	}

	if ctx.EffectiveMessage.Chat.Type == gotgbot.ChatTypeGroup || ctx.EffectiveMessage.Chat.Type == gotgbot.ChatTypeSupergroup {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("start-message-group", map[string]any{"botName": botUser.FirstName}), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
				Text: i18n("start-button"),
				Url:  fmt.Sprintf("https://t.me/%s?start=start", botUser.Username),
			}}}},
		})
		return nil
	}

	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("start-message", map[string]any{
		"userFirstName": ctx.EffectiveUser.FirstName,
		"botName":       botUser.FirstName,
	}), &gotgbot.SendMessageOpts{
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyParameters:    &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup:        createStartKeyboard(i18n),
	})

	return nil
}

func startCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	botUser, err := b.GetMe(nil)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()
	firstName := ctx.CallbackQuery.From.FirstName

	_, _, _ = b.EditMessageText(i18n("start-message", map[string]any{"userFirstName": firstName, "botName": botUser.FirstName}), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup:        createStartKeyboard(i18n),
	})

	return nil
}

func privacyHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	botUser, err := b.GetMe(nil)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if ctx.EffectiveMessage.Chat.Type == gotgbot.ChatTypeGroup || ctx.EffectiveMessage.Chat.Type == gotgbot.ChatTypeSupergroup {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("privacy-policy-group"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
				Text: i18n("privacy-policy-button"),
				Url:  fmt.Sprintf("https://t.me/%s?start=privacy", botUser.Username),
			}}}},
		})
		return nil
	}

	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("privacy-policy-private"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
			Text: i18n("about-your-data-button"), CallbackData: "aboutYourData",
		}}}},
	})

	return nil
}

func privacyCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n("privacy-policy-private"), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: i18n("about-your-data-button"), CallbackData: "aboutYourData"}},
			{{Text: i18n("back-button"), CallbackData: "about"}},
		}},
	})
	return nil
}

func aboutMenuCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n("about-message"), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: i18n("donation-button"), Url: "https://ko-fi.com/ruizlenato"}, {Text: i18n("news-channel-button"), Url: "https://t.me/SmudgeLordChannel"}},
			{{Text: i18n("privacy-policy-button"), CallbackData: "privacy"}},
			{{Text: i18n("back-button"), CallbackData: "start"}},
		}},
	})
	return nil
}

func aboutYourDataCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n("about-your-data"), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: i18n("back-button"), CallbackData: "privacy"},
		}}},
	})
	return nil
}

func helpMenuCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n("help-message"), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: utils.GetHelpKeyboard(i18n),
		},
	})
	return nil
}

func helpMessageCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	module := strings.ReplaceAll(ctx.CallbackQuery.Data, "helpMessage ", "")
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n(fmt.Sprintf("%s-help", module)), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: i18n("back-button"), CallbackData: "helpMenu"},
		}}},
	})
	return nil
}

type inlineArticle struct {
	id             string
	title          string
	description    string
	messageContent string
}

func filterArticles(articles []inlineArticle, query string) []inlineArticle {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return articles
	}

	lowerQuery := strings.ToLower(trimmedQuery)
	filtered := make([]inlineArticle, 0, len(articles))
	for _, article := range articles {
		if strings.Contains(strings.ToLower(article.title), lowerQuery) {
			filtered = append(filtered, article)
		}
	}
	return filtered
}

func menuInline(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.InlineQuery == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	articles := []inlineArticle{
		{id: "media", title: i18n("media-inline-handler"), description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "track"}), messageContent: fmt.Sprintf("<b>%s</b>: %s", i18n("media-inline-handler"), i18n("media-inline-help"))},
		{id: "weather", title: html.UnescapeString(i18n("weather-inline-handler")), description: i18n("weather-inline-description"), messageContent: fmt.Sprintf("<b>%s</b>: %s", i18n("weather-inline-handler"), i18n("weather-inline-description"))},
		{id: "track", title: "LastFM Music", description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "track"}), messageContent: i18n("loading")},
		{id: "album", title: "LastFM Album", description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "album"}), messageContent: i18n("loading")},
		{id: "artist", title: "LastFM Artist", description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "artist"}), messageContent: i18n("loading")},
	}

	filtered := filterArticles(articles, ctx.InlineQuery.Query)
	results := make([]gotgbot.InlineQueryResult, 0, len(filtered))

	for _, article := range filtered {
		emoji := ""
		switch article.id {
		case "track":
			emoji = "🎵"
		case "album":
			emoji = "💿"
		case "artist":
			emoji = "🎙"
		}

		replyMarkup := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: emoji, CallbackData: "NONE"}}}}
		res := gotgbot.InlineQueryResultArticle{
			Id:          article.id,
			Title:       article.title,
			Description: article.description,
			InputMessageContent: gotgbot.InputTextMessageContent{
				MessageText: article.messageContent,
				ParseMode:   gotgbot.ParseModeHTML,
			},
			ReplyMarkup: replyMarkup,
		}
		results = append(results, res)
	}

	if len(results) > 0 {
		cacheTime := int64(0)
		_, _ = b.AnswerInlineQuery(ctx.InlineQuery.Id, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
	}

	return nil
}

func inlineSend(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.ChosenInlineResult == nil {
		return nil
	}

	switch ctx.ChosenInlineResult.ResultId {
	case "track", "artist", "album":
		return lastfm.LastfmInline(b, ctx)
	case "media":
		return medias.MediasInline(b, ctx)
	default:
		if location, found := strings.CutPrefix(ctx.ChosenInlineResult.ResultId, "weather-"); found {
			return misc.WeatherInline(b, ctx, location)
		}
		return nil
	}
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewChosenInlineResult(choseninlineresult.All, inlineSend))
	dispatcher.AddHandler(handlers.NewInlineQuery(nil, menuInline))
	dispatcher.AddHandler(handlers.NewCommand("start", startHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("start"), startCallback))
	dispatcher.AddHandler(handlers.NewCommand("privacy", privacyHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("privacy"), privacyCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("about"), aboutMenuCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("aboutYourData"), aboutYourDataCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("helpMenu"), helpMenuCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("helpMessage"), helpMessageCallback))
}
