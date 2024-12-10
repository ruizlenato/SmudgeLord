package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
)

func handlerDisable(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	contains := func(array []string, str string) bool {
		for _, item := range array {
			if item == str {
				return true
			}
		}
		return false
	}

	command := strings.Split(message.Args(), " ")[0]
	if !contains(handlers.DisableableCommands, command) {
		_, err := message.Reply(i18n("enable-commands-usage"))
		return err
	}

	if handlers.CheckDisabledCommand(command) {
		_, err := message.Reply(i18n("command-already-disabled",
			map[string]interface{}{
				"command": command,
			}))
		return err
	}

	query := "INSERT INTO commandsDisabled (command) VALUES (?);"
	_, err := database.DB.Exec(query, command)
	if err != nil {
		fmt.Printf("Error inserting command: %v\n", err)
		return err
	}
	_, err = message.Reply(i18n("command-disabled",
		map[string]interface{}{
			"command": command,
		}))
	return err
}

func handlerEnable(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	contains := func(array []string, str string) bool {
		for _, item := range array {
			if item == str {
				return true
			}
		}
		return false
	}

	command := strings.Split(message.Args(), " ")[0]
	if !contains(handlers.DisableableCommands, command) {
		_, err := message.Reply("command not disableable")
		return err
	}

	if !handlers.CheckDisabledCommand(command) {
		_, err := message.Reply(i18n("command-already-enabled",
			map[string]interface{}{
				"command": command,
			}))
		return err
	}

	query := "DELETE FROM commandsDisabled WHERE command = ?;"
	_, err := database.DB.Exec(query, command)
	if err != nil {
		fmt.Printf("Error deleting command: %v\n", err)
		return err
	}
	_, err = message.Reply(i18n("command-enabled",
		map[string]interface{}{
			"command": command,
		}))

	return err
}

func handlerDisableableCommands(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	text := i18n("disableables-commands")
	for _, command := range handlers.DisableableCommands {
		text += "\n- " + "<code>" + command + "</code>"
	}
	_, err := message.Reply(text)

	return err
}

func createConfigKeyboard(i18n func(string, ...map[string]interface{}) string) telegram.ReplyMarkup {
	return telegram.Button{}.Keyboard(
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("medias"),
				"mediaConfig",
			),
		),
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("language-flag")+i18n("language-button"),
				"languageMenu",
			),
		),
	)
}

func handlerConfig(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	keyboard := createConfigKeyboard(i18n)

	_, err := message.Reply(i18n("config-message"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})

	return err
}

func callbackConfig(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	keyboard := createConfigKeyboard(i18n)
	_, err := update.Edit(i18n("config-message"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})
	return err
}

func callbackLanguageMenu(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	buttons := telegram.Button{}.Keyboard()
	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangBundles[lang]
		if !ok {
			log.Fatalf("Language '%s' not found in the cache.", lang)
		}
		languageFlag, _, _ := loaded.FormatMessage("language-flag")
		languageName, _, _ := loaded.FormatMessage("language-name")

		buttons.Rows = append(buttons.Rows, telegram.Button{}.Row(telegram.Button{}.Data(
			languageFlag+languageName,
			"setLang "+lang,
		)))
	}

	row := database.DB.QueryRow("SELECT language FROM chats WHERE id = ?;", update.ChatID)
	if update.ChatType() == telegram.EntityUser {
		row = database.DB.QueryRow("SELECT language FROM users WHERE id = ?;", update.ChatID)
	}
	var language string
	err := row.Scan(&language)
	if err != nil {
		log.Print("[start/languageMenu] Error querying user: ", err)
	}

	_, err = update.Edit(i18n("language-menu"), &telegram.SendOptions{
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
			i18n("back-button"),
			"start",
		)))
	} else {
		buttons.Rows = append(buttons.Rows, telegram.Button{}.Row(telegram.Button{}.Data(
			i18n("back-button"),
			"config",
		)))
	}

	_, err = update.Edit(i18n("language-changed"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: buttons,
	})
	return err
}

func callbackMediaConfig(update *telegram.CallbackQuery) error {
	var mediasCaption bool
	var mediasAuto bool

	err := database.DB.QueryRow("SELECT mediasCaption FROM chats WHERE id = ?;", update.ChatID).Scan(&mediasCaption)
	if err != nil {
		return err
	}

	err = database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", update.ChatID).Scan(&mediasAuto)
	if err != nil {
		return err
	}

	configType := strings.ReplaceAll(update.DataString(), "mediaConfig ", "")
	if configType != "mediaConfig" {
		query := fmt.Sprintf("UPDATE chats SET %s = ? WHERE id = ?;", configType)
		var err error
		switch configType {
		case "mediasCaption":
			mediasCaption = !mediasCaption
			_, err = database.DB.Exec(query, mediasCaption, update.ChatID)
		case "mediasAuto":
			mediasAuto = !mediasAuto
			_, err = database.DB.Exec(query, mediasAuto, update.ChatID)
		}
		if err != nil {
			return err
		}
	}

	i18n := localization.Get(update)

	state := func(mediasAuto bool) string {
		if mediasAuto {
			return "✅"
		}
		return "☑️"
	}

	keyboard := telegram.Button{}.Keyboard(
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("caption-button"),
				"ieConfig mediasCaption",
			),
			telegram.Button{}.Data(
				state(mediasCaption),
				"mediaConfig mediasCaption",
			),
		),
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("automatic-button"),
				"ieConfig mediasAuto",
			),
			telegram.Button{}.Data(
				state(mediasAuto),
				"mediaConfig mediasAuto",
			),
		),
	)

	keyboard.Rows = append(keyboard.Rows, telegram.Button{}.Row(telegram.Button{}.Data(
		i18n("back-button"),
		"configMenu",
	)))

	_, err = update.Edit(i18n("config-medias"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})

	return err
}

func callbackExplainConfig(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	ieConfig := strings.ReplaceAll(update.DataString(), "ieConfig medias", "")
	_, err := update.Answer(i18n("medias."+strings.ToLower(ieConfig)+"Help"), &telegram.CallbackOptions{
		Alert: true,
	})
	return err
}

func Load(client *telegram.Client) {
	client.On("command:disable", handlers.HandleCommand(handlerDisable), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("command:enable", handlers.HandleCommand(handlerEnable), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("command:disableable", handlers.HandleCommand(handlerDisableableCommands), telegram.Filter{Func: helpers.IsAdmin})
	client.On("command:config", handlers.HandleCommand(handlerConfig), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("callback:config", callbackConfig, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:languageMenu", callbackLanguageMenu, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:setLang", callbackLanguageSet, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:mediaConfig", callbackMediaConfig, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:ieConfig", callbackExplainConfig, telegram.Filter{Func: helpers.IsAdmin})
}
