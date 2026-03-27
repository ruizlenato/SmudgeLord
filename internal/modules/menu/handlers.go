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

func createStartKeyboardGotgbot(i18n func(string, ...map[string]any) string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
		{{Text: i18n("about-button"), CallbackData: "about"}, {Text: fmt.Sprintf("%s %s", i18n("language-flag"), i18n("language-button")), CallbackData: "languageMenu"}},
		{{Text: i18n("help-button"), CallbackData: "helpMenu"}},
	}}
}

func startHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	botUser, err := b.GetMe(nil)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if messageFields := strings.Fields(ctx.EffectiveMessage.GetText()); len(messageFields) > 1 && messageFields[1] == "privacy" {
		return privacyHandlerGotgbot(b, ctx)
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
		ReplyMarkup:        createStartKeyboardGotgbot(i18n),
	})

	return nil
}

func startCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
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
		ReplyMarkup:        createStartKeyboardGotgbot(i18n),
	})

	return nil
}

func privacyHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
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

func privacyCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
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

func aboutMenuCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
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

func aboutYourDataCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
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

func helpMenuCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n("help-message"), &gotgbot.EditMessageTextOpts{
		ChatId:             chat.Id,
		MessageId:          msgID,
		ParseMode:          gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: utils.GetHelpKeyboardGotgbot(i18n),
		},
	})
	return nil
}

func helpMessageCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
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

type inlineArticleGotgbot struct {
	id             string
	title          string
	description    string
	messageContent string
}

func filterArticlesGotgbot(articles []inlineArticleGotgbot, query string) []inlineArticleGotgbot {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return articles
	}

	lowerQuery := strings.ToLower(trimmedQuery)
	filtered := make([]inlineArticleGotgbot, 0, len(articles))
	for _, article := range articles {
		if strings.Contains(strings.ToLower(article.title), lowerQuery) {
			filtered = append(filtered, article)
		}
	}
	return filtered
}

func menuInlineGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.InlineQuery == nil {
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	articles := []inlineArticleGotgbot{
		{id: "media", title: i18n("media-inline-handler"), description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "track"}), messageContent: fmt.Sprintf("<b>%s</b>: %s", i18n("media-inline-handler"), i18n("media-inline-help"))},
		{id: "weather", title: html.UnescapeString(i18n("weather-inline-handler")), description: i18n("weather-inline-description"), messageContent: fmt.Sprintf("<b>%s</b>: %s", i18n("weather-inline-handler"), i18n("weather-inline-description"))},
		{id: "track", title: "LastFM Music", description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "track"}), messageContent: i18n("loading")},
		{id: "album", title: "LastFM Album", description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "album"}), messageContent: i18n("loading")},
		{id: "artist", title: "LastFM Artist", description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "artist"}), messageContent: i18n("loading")},
	}

	filtered := filterArticlesGotgbot(articles, ctx.InlineQuery.Query)
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

func inlineSendGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
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
	dispatcher.AddHandler(handlers.NewChosenInlineResult(choseninlineresult.All, inlineSendGotgbot))
	dispatcher.AddHandler(handlers.NewInlineQuery(nil, menuInlineGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("start", startHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("start"), startCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("privacy", privacyHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("privacy"), privacyCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("about"), aboutMenuCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("aboutYourData"), aboutYourDataCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("helpMenu"), helpMenuCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("helpMessage"), helpMessageCallbackGotgbot))
}
