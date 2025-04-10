package menu

import (
	"fmt"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func createStartKeyboard(i18n func(string, ...map[string]interface{}) string) telegram.ReplyMarkup {
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

func handlerStart(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if messageFields := strings.Fields(message.Text()); len(messageFields) > 1 && messageFields[1] == "privacy" {
		return handlerPrivacy(message)
	}

	if message.ChatType() == telegram.EntityUser {
		_, err := message.Reply(i18n("start-message",
			map[string]interface{}{
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
		map[string]interface{}{
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

func callbackStart(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	_, err := update.Edit(i18n("start-message",
		map[string]interface{}{
			"userFirstName": update.Sender.FirstName,
			"botName":       update.Client.Me().FirstName,
		}), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: createStartKeyboard(i18n),
	})

	return err
}

func createPrivacyKeyboard(i18n func(string, ...map[string]interface{}) string) telegram.ReplyMarkup {
	return telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("about-your-data-button"),
				"aboutYourData",
			),
		),
	)
}

func handlerPrivacy(message *telegram.NewMessage) error {
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

func callbackPrivacy(update *telegram.CallbackQuery) error {
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

func callbackAboutYourData(update *telegram.CallbackQuery) error {
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

func callbackAboutMenu(update *telegram.CallbackQuery) error {
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

func callbackHelpMenu(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	_, err := update.Edit(i18n("help-message"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: utils.GetHelpKeyboard(i18n),
	})
	return err
}

func callbackHelpMessage(update *telegram.CallbackQuery) error {
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

func Load(client *telegram.Client) {
	client.On("command:start", handlers.HandleCommand(handlerStart))
	client.On("callback:start", callbackStart)
	client.On("command:privacy", handlers.HandleCommand(handlerPrivacy))
	client.On("callback:privacy", callbackPrivacy)
	client.On("callback:aboutYourData", callbackAboutYourData)
	client.On("callback:aboutMenu", callbackAboutMenu)
	client.On("callback:helpMenu", callbackHelpMenu)
	client.On("callback:helpMessage", callbackHelpMessage)
}
