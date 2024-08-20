package menu

import (
	"fmt"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func createStartKeyboard(i18n func(string) string) telegram.ReplyMarkup {
	return telegram.Button{}.Keyboard(
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("button.about"),
				"aboutMenu",
			),
			telegram.Button{}.Data(
				fmt.Sprintf("%s %s", i18n("language.flag"), i18n("button.language")),
				"languageMenu",
			),
		),
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("button.help"),
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
		_, err := message.Reply(fmt.Sprintf(i18n("menu.startMsg"), message.Sender.FirstName, message.Client.Me().FirstName),
			telegram.SendOptions{
				ParseMode:   telegram.HTML,
				ReplyMarkup: createStartKeyboard(i18n),
			})
		return err
	}

	_, err := message.Reply(fmt.Sprintf(i18n("menu.startGroupMsg"), message.Client.Me().FirstName),
		telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyMarkup: telegram.Button{}.Keyboard(
				telegram.Button{}.Row(
					telegram.Button{}.URL(
						i18n("button.start"),
						fmt.Sprintf("https://t.me/%s?start=start", message.Client.Me().Username),
					))),
		})

	return err
}

func callbackStart(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	_, err := update.Edit(fmt.Sprintf(i18n("menu.startMsg"), update.Sender.FirstName, update.Client.Me().FirstName), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: createStartKeyboard(i18n),
	})

	return err
}

func createPrivacyKeyboard(i18n func(string) string) telegram.ReplyMarkup {
	return telegram.Button{}.Keyboard(
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("button.aboutYourData"),
				"aboutYourData",
			),
		),
	)
}

func handlerPrivacy(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if message.ChatType() == telegram.EntityUser {
		_, err := message.Reply(i18n("menu.privacyMessage"),
			telegram.SendOptions{
				ParseMode:   telegram.HTML,
				ReplyMarkup: createPrivacyKeyboard(i18n),
			})
		return err
	}
	_, err := message.Reply(i18n("menu.privacyGroupMsg"),
		telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyMarkup: telegram.Button{}.Keyboard(
				telegram.Button{}.Row(
					telegram.Button{}.URL(
						i18n("button.aboutYourData"),
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
	keyboard.(*telegram.ReplyInlineMarkup).Rows = append(keyboard.(*telegram.ReplyInlineMarkup).Rows, telegram.Button{}.Row(telegram.Button{}.Data(
		i18n("button.back"),
		"start",
	)))
	_, err := update.Edit(i18n("menu.privacyMessage"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})
	return err
}

func callbackAboutYourData(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	_, err := update.Edit(i18n("menu.yourDataMsg"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.Button{}.Keyboard(
			telegram.Button{}.Row(
				telegram.Button{}.Data(
					i18n("button.back"),
					"privacy",
				),
			),
		),
	})
	return err
}

func callbackAboutMenu(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	_, err := update.Edit(i18n("menu.aboutMsg"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.Button{}.Keyboard(
			telegram.Button{}.Row(
				telegram.Button{}.URL(
					i18n("button.donation"),
					"https://ko-fi.com/ruizlenato",
				),
				telegram.Button{}.URL(
					i18n("button.newsChannel"),
					"https://t.me/SmudgeLordChannel",
				),
			),
			telegram.Button{}.Row(
				telegram.Button{}.Data(
					i18n("button.privacy"),
					"privacy",
				),
			),
			telegram.Button{}.Row(
				telegram.Button{}.Data(
					i18n("button.back"),
					"start",
				),
			),
		),
	})
	return err
}

func callbackHelpMenu(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	_, err := update.Edit(i18n("menu.helpMsg"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: utils.GetHelpKeyboard(i18n),
	})
	return err
}

func callbackHelpMessage(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	module := strings.TrimPrefix(update.DataString(), "helpMessage ")
	_, err := update.Edit(i18n(module+".help"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.Button{}.Keyboard(
			telegram.Button{}.Row(
				telegram.Button{}.Data(
					i18n("button.back"),
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
