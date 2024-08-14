package menu

import (
	"fmt"
	"log"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
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
				i18n("button.privacy"),
				"privacy",
			),
		),
	)
}

func handlerStart(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if messageFields := strings.Fields(message.Text()); len(messageFields) > 1 && messageFields[1] == "privacy" {
		return handlerPrivacy(message)
	}

	if message.ChatType() == "user" {
		_, err := message.Reply(fmt.Sprintf(i18n("menu.start-message"), message.Sender.FirstName, message.Client.Me().FirstName),
			telegram.SendOptions{
				ParseMode:   telegram.HTML,
				ReplyMarkup: createStartKeyboard(i18n),
			})
		return err
	}

	_, err := message.Reply(fmt.Sprintf(i18n("menu.start-message-group"), message.Client.Me().FirstName),
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

	_, err := update.Edit(fmt.Sprintf(i18n("menu.start-message"), update.Sender.FirstName, update.Client.Me().FirstName), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: createStartKeyboard(i18n),
	})

	return err
}

func callbackLanguageMenu(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	buttons := telegram.Button{}.Keyboard()
	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangCache[lang]
		if !ok {
			log.Fatalf("Language '%s' not found in the cache.", lang)
		}

		buttons.Rows = append(buttons.Rows, telegram.Button{}.Row(telegram.Button{}.Data(
			localization.GetStringFromNestedMap(loaded, "language.flag")+
				localization.GetStringFromNestedMap(loaded, "language.name"),
			"setLang "+lang,
		)))
	}

	row := database.DB.QueryRow("SELECT language FROM chats WHERE id = ?;", update.ChatID)
	if update.ChatType() == "user" {
		row = database.DB.QueryRow("SELECT language FROM users WHERE id = ?;", update.ChatID)
	}
	var language string
	err := row.Scan(&language)
	if err != nil {
		log.Print("[start/languageMenu] Error querying user: ", err)
	}

	_, err = update.Edit(fmt.Sprintf(i18n("menu.language-mesage"), i18n("language.flag"), i18n("language.name")), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: buttons,
	})
	return err
}

func callbackLanguageSet(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	lang := strings.ReplaceAll(update.DataString(), "setLang ", "")

	dbQuery := "UPDATE chats SET language = ? WHERE id = ?;"
	if update.ChatType() == "user" {
		dbQuery = "UPDATE users SET language = ? WHERE id = ?;"
	}
	_, err := database.DB.Exec(dbQuery, lang, update.ChatID)
	if err != nil {
		log.Print("[start/languageSet] Error updating language: ", err)
	}

	buttons := telegram.Button{}.Keyboard()

	if update.ChatType() == "user" {
		buttons.Rows = append(buttons.Rows, telegram.Button{}.Row(telegram.Button{}.Data(
			i18n("button.back"),
			"start",
		)))
	}

	_, err = update.Edit(i18n("menu.language-changed-successfully"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: buttons,
	})
	return err
}

func createPrivacyKeyboard(i18n func(string) string) telegram.ReplyMarkup {
	return telegram.Button{}.Keyboard(
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("button.about-your-data"),
				"aboutYourData",
			),
		),
	)
}

func handlerPrivacy(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if message.ChatType() == "user" {
		_, err := message.Reply(i18n("menu.privacy-message"),
			telegram.SendOptions{
				ParseMode:   telegram.HTML,
				ReplyMarkup: createPrivacyKeyboard(i18n),
			})
		return err
	}
	_, err := message.Reply(i18n("menu.privacy-group-message"),
		telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyMarkup: telegram.Button{}.Keyboard(
				telegram.Button{}.Row(
					telegram.Button{}.URL(
						i18n("button.about-your-data"),
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
	_, err := update.Edit(i18n("menu.privacy-message"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})
	return err
}

func callbackAboutYourData(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	_, err := update.Edit(i18n("menu.yourData-message"), &telegram.SendOptions{
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
	_, err := update.Edit(i18n("menu.yourData-message"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyMarkup: telegram.Button{}.Keyboard(
			telegram.Button{}.Row(
				telegram.Button{}.URL(
					i18n("button.donation"),
					"https://ko-fi.com/ruizlenato",
				),
				telegram.Button{}.URL(
					i18n("button.news-channel"),
					"https://t.me/SmudgeLordChannel",
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

func Load(client *telegram.Client) {
	client.On("command:start", handlers.HandleCommand(handlerStart))
	client.On("callback:start", callbackStart)
	client.On("callback:languageMenu", callbackLanguageMenu)
	client.On("callback:setLang", callbackLanguageSet)
	client.On("command:privacy", handlers.HandleCommand(handlerPrivacy))
	client.On("callback:privacy", callbackPrivacy)
	client.On("callback:aboutYourData", callbackAboutYourData)
	client.On("callback:aboutMenu", callbackAboutMenu)
}
