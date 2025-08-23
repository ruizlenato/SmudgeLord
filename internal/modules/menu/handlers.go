package menu

import (
	"fmt"
	"html"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/lastfm"
	"github.com/ruizlenato/smudgelord/internal/modules/medias"
	"github.com/ruizlenato/smudgelord/internal/modules/misc"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func createStartKeyboard(i18n func(string, ...map[string]any) string) telegram.ReplyMarkup {
	return telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("about-button"),
				"aboutMenu",
			),
			telegram.ButtonBuilder{}.Data(
				fmt.Sprintf("%s %s", i18n("language-flag"), i18n("language-button")),
				"languageMenu",
			),
		),
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("help-button"),
				"helpMenu",
			),
		),
	)
}

func startHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if messageFields := strings.Fields(message.Text()); len(messageFields) > 1 && messageFields[1] == "privacy" {
		return privacyHandler(message)
	}
	if messageFields := strings.Fields(message.Text()); len(messageFields) > 1 && messageFields[1] == "setuser" {
		return lastfm.SetUserHandler(message)
	}

	if message.ChatType() == telegram.EntityUser {
		_, err := message.Reply(i18n("start-message",
			map[string]any{
				"userFirstName": message.Sender.FirstName,
				"botName":       message.Client.Me().FirstName,
			}),
			telegram.SendOptions{
				ParseMode:   telegram.HTML,
				ReplyMarkup: createStartKeyboard(i18n),
			})
		return err
	}

	_, err := message.Reply(i18n("start-message-group",
		map[string]any{
			"botName": message.Client.Me().FirstName,
		}),
		telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
				telegram.ButtonBuilder{}.Row(
					telegram.ButtonBuilder{}.URL(
						i18n("start-button"),
						fmt.Sprintf("https://t.me/%s?start=start", message.Client.Me().Username),
					))),
		})

	return err
}

func startCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	_, err := update.Edit(i18n("start-message",
		map[string]any{
			"userFirstName": update.Sender.FirstName,
			"botName":       update.Client.Me().FirstName,
		}), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: createStartKeyboard(i18n),
	})

	return err
}

func createPrivacyKeyboard(i18n func(string, ...map[string]any) string) telegram.ReplyMarkup {
	return telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("about-your-data-button"),
				"aboutYourData",
			),
		),
	)
}

func privacyHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if message.ChatType() == telegram.EntityUser {
		_, err := message.Reply(i18n("privacy-policy-private"),
			telegram.SendOptions{
				ParseMode:   telegram.HTML,
				ReplyMarkup: createPrivacyKeyboard(i18n),
			})
		return err
	}
	_, err := message.Reply(i18n("privacy-policy-group"),
		telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
				telegram.ButtonBuilder{}.Row(
					telegram.ButtonBuilder{}.URL(
						i18n("about-your-data-button"),
						fmt.Sprintf("https://t.me/%s?start=privacy", message.Client.Me().Username),
					),
				),
			),
		})
	return err
}

func privacyCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	keyboard := createPrivacyKeyboard(i18n)
	keyboard.(*telegram.ReplyInlineMarkup).Rows = append(keyboard.(*telegram.ReplyInlineMarkup).Rows, telegram.ButtonBuilder{}.Row(telegram.ButtonBuilder{}.Data(
		i18n("back-button"),
		"aboutMenu",
	)))
	_, err := update.Edit(i18n("privacy-policy-private"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})
	return err
}

func aboutYourDataCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	_, err := update.Edit(i18n("about-your-data"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
			telegram.ButtonBuilder{}.Row(
				telegram.ButtonBuilder{}.Data(
					i18n("back-button"),
					"privacy",
				),
			),
		),
	})
	return err
}

func aboutMenuCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	_, err := update.Edit(i18n("about-message"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
			telegram.ButtonBuilder{}.Row(
				telegram.ButtonBuilder{}.URL(
					i18n("donation-button"),
					"https://ko-fi.com/ruizlenato",
				),
				telegram.ButtonBuilder{}.URL(
					i18n("news-channel-button"),
					"https://t.me/SmudgeLordChannel",
				),
			),
			telegram.ButtonBuilder{}.Row(
				telegram.ButtonBuilder{}.Data(
					i18n("privacy-policy-button"),
					"privacy",
				),
			),
			telegram.ButtonBuilder{}.Row(
				telegram.ButtonBuilder{}.Data(
					i18n("back-button"),
					"start",
				),
			),
		),
	})
	return err
}

func helpMenuCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	_, err := update.Edit(i18n("help-message"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: utils.GetHelpKeyboard(i18n),
	})
	return err
}

func helpMessageCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	module := strings.TrimPrefix(update.DataString(), "helpMessage ")
	_, err := update.Edit(i18n(module+"-help"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
			telegram.ButtonBuilder{}.Row(
				telegram.ButtonBuilder{}.Data(
					i18n("back-button"),
					"helpMenu",
				),
			),
		),
	})
	return err
}

type inlineArticle struct {
	title       string
	description string
	text        string
	options     *telegram.ArticleOptions
}

func menuInline(i *telegram.InlineQuery) error {
	builder := i.Builder()
	i18n := localization.Get(i)

	articles := []inlineArticle{
		{
			title:       html.UnescapeString(i18n("weather-inline-handler")),
			description: i18n("weather-inline-help"),
			text:        fmt.Sprintf("<b>%s</b>: %s", i18n("weather-inline-handler"), i18n("weather-inline-help")),
			options: &telegram.ArticleOptions{
				ParseMode: telegram.HTML,
				ReplyMarkup: telegram.NewKeyboard().AddRow(
					telegram.Button.SwitchInline(i18n("run-switch-inline", map[string]any{"command": i18n("weather")}), true, i18n("weather")),
				).Build(),
			},
		},
		{
			title:       "LastFM Music",
			description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "track"}),
			text:        "BALD!",
			options: &telegram.ArticleOptions{
				ID:        "track",
				ParseMode: telegram.HTML,
				ReplyMarkup: telegram.NewKeyboard().AddRow(
					telegram.Button.Data("🎵", "NONE"),
				).Build(),
			},
		},
		{
			title:       "LastFM Album",
			description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "album"}),
			text:        "BALD!",
			options: &telegram.ArticleOptions{
				ID:        "album",
				ParseMode: telegram.HTML,
				ReplyMarkup: telegram.NewKeyboard().AddRow(
					telegram.Button.Data("💿", "NONE"),
				).Build(),
			},
		},
		{
			title:       "LastFM Artist",
			description: i18n("lastfm-inline-description", map[string]any{"lastfmType": "artist"}),
			text:        "BALD!",
			options: &telegram.ArticleOptions{
				ID:        "artist",
				ParseMode: telegram.HTML,
				ReplyMarkup: telegram.NewKeyboard().AddRow(
					telegram.Button.Data("🎙", "NONE"),
				).Build(),
			},
		},
	}

	filteredArticles := filterArticles(articles, i.Query)

	for _, article := range filteredArticles {
		builder.Article(
			article.title,
			article.description,
			article.text,
			article.options,
		)
	}

	_, err := i.Answer(builder.Results(), telegram.InlineSendOptions{
		CacheTime: 0,
	})
	return err
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

func inlineSend(m *telegram.InlineSend) error {
	switch {
	case m.ID == "track" || m.ID == "artist" || m.ID == "album":
		return lastfm.LastfmInline(m, m.ID)
	case strings.HasPrefix(m.ID, "weather-"):
		location := strings.TrimPrefix(m.ID, "weather-")
		return misc.WeatherInline(m, location)
	case m.ID == "media":
		return medias.MediasInline(m)
	default:
		return nil
	}
}

func Load(client *telegram.Client) {
	client.On("command:start", handlers.HandleCommand(startHandler))
	client.On("callback:^start", startCallback)
	client.On("command:privacy", handlers.HandleCommand(privacyHandler))
	client.On("callback:^privacy", privacyCallback)
	client.On("callback:^aboutYourData", aboutYourDataCallback)
	client.On("callback:aboutMenu", aboutMenuCallback)
	client.On("callback:^helpMenu", helpMenuCallback)
	client.On("callback:^helpMessage", helpMessageCallback)
	client.AddInlineHandler(telegram.OnInlineQuery, menuInline)
	client.AddInlineSendHandler(inlineSend)
}
