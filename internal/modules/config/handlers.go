package config

import (
	"fmt"
	"log"
	"log/slog"
	"strings"

	"slices"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func disableHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	contains := func(array []string, str string) bool {
		return slices.Contains(array, str)
	}

	command := strings.Split(message.Args(), " ")[0]
	if !contains(handlers.DisableableCommands, command) {
		_, err := message.Reply(i18n("enable-commands-usage"))
		return err
	}

	if handlers.CheckDisabledCommand(message, command) {
		_, err := message.Reply(i18n("command-already-disabled",
			map[string]any{
				"command": command,
			}))
		return err
	}

	query := "INSERT INTO commandsDisabled (chat_id, command) VALUES (?, ?);"
	_, err := database.DB.Exec(query, message.Chat.ID, command)
	if err != nil {
		fmt.Printf("Error inserting command: %v\n", err)
		return err
	}
	_, err = message.Reply(i18n("command-disabled",
		map[string]any{
			"command": command,
		}))
	return err
}

func enableHandler(message *telegram.NewMessage) error {
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

	if !handlers.CheckDisabledCommand(message, command) {
		_, err := message.Reply(i18n("command-already-enabled",
			map[string]any{
				"command": command,
			}))
		return err
	}

	query := "DELETE FROM commandsDisabled WHERE command = ? AND chat_id = ?;"
	_, err := database.DB.Exec(query, command, message.Chat.ID)
	if err != nil {
		fmt.Printf("Error deleting command: %v\n", err)
		return err
	}
	_, err = message.Reply(i18n("command-enabled",
		map[string]any{
			"command": command,
		}))

	return err
}

func disableableHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	text := i18n("disableables-commands")
	for _, command := range handlers.DisableableCommands {
		text += "\n- " + "<code>" + command + "</code>"
	}
	_, err := message.Reply(text)

	return err
}

func disabledHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	text := i18n("disabled-commands")
	rows, err := database.DB.Query("SELECT command FROM commandsDisabled WHERE chat_id = ?", message.Chat.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var commands []string
	for rows.Next() {
		var command string
		if err := rows.Scan(&command); err != nil {
			return err
		}
		commands = append(commands, command)
	}

	if len(commands) == 0 {
		_, err := message.Reply(i18n("no-disabled-commands"))
		return err
	}
	for _, command := range commands {
		text += "\n- <code>" + command + "</code>"
	}
	_, err = message.Reply(text)
	return err
}

func createConfigKeyboard(i18n func(string, ...map[string]any) string) telegram.ReplyMarkup {
	return telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("medias"),
				"mediaConfig",
			),
		),
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("language-flag")+i18n("language-button"),
				"languageMenu",
			),
		),
	)
}

func configHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	keyboard := createConfigKeyboard(i18n)

	_, err := message.Reply(i18n("config-message"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})

	return err
}

func configCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	keyboard := createConfigKeyboard(i18n)
	_, err := update.Edit(i18n("config-message"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})
	return err
}

func languageMenuCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)

	buttons := telegram.ButtonBuilder{}.Keyboard()
	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangBundles[lang]
		if !ok {
			log.Fatalf("Language '%s' not found in the cache.", lang)
		}
		languageFlag, _, _ := loaded.FormatMessage("language-flag")
		languageName, _, _ := loaded.FormatMessage("language-name")

		buttons.Rows = append(buttons.Rows, telegram.ButtonBuilder{}.Row(telegram.ButtonBuilder{}.Data(
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
		slog.Error(
			"Error querying user",
			"error", err.Error(),
		)
	}

	_, err = update.Edit(i18n("language-menu"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: buttons,
	})
	return err
}

func languageSetCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	lang := strings.ReplaceAll(update.DataString(), "setLang ", "")

	dbQuery := "UPDATE chats SET language = ? WHERE id = ?;"
	if update.ChatType() == "user" {
		dbQuery = "UPDATE users SET language = ? WHERE id = ?;"
	}
	_, err := database.DB.Exec(dbQuery, lang, update.ChatID)
	if err != nil {
		slog.Error(
			"Error updating language",
			"lang", lang,
			"chat_id", update.ChatID,
			"error", err.Error(),
		)
	}

	buttons := telegram.ButtonBuilder{}.Keyboard()

	if update.ChatType() == "user" {
		buttons.Rows = append(buttons.Rows, telegram.ButtonBuilder{}.Row(telegram.ButtonBuilder{}.Data(
			i18n("back-button"),
			"start",
		)))
	} else {
		buttons.Rows = append(buttons.Rows, telegram.ButtonBuilder{}.Row(telegram.ButtonBuilder{}.Data(
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

func mediaConfigCallback(update *telegram.CallbackQuery) error {
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

	keyboard := telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("caption-button"),
				"ieConfig mediasCaption",
			),
			telegram.ButtonBuilder{}.Data(
				state(mediasCaption),
				"mediaConfig mediasCaption",
			),
		),
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("automatic-button"),
				"ieConfig mediasAuto",
			),
			telegram.ButtonBuilder{}.Data(
				state(mediasAuto),
				"mediaConfig mediasAuto",
			),
		),
	)

	keyboard.Rows = append(keyboard.Rows, telegram.ButtonBuilder{}.Row(telegram.ButtonBuilder{}.Data(
		i18n("back-button"),
		"configMenu",
	)))

	_, err = update.Edit(i18n("config-medias"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: keyboard,
	})

	return err
}

func explainConfigCallback(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	ieConfig := strings.ReplaceAll(update.DataString(), "ieConfig medias", "")
	_, err := update.Answer(i18n("medias."+strings.ToLower(ieConfig)+"Help"), &telegram.CallbackOptions{
		Alert: true,
	})
	return err
}

func Load(client *telegram.Client) {
	utils.SotreHelp("config")
	client.On("command:disable", handlers.HandleCommand(disableHandler), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("command:enable", handlers.HandleCommand(enableHandler), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("command:disableable", handlers.HandleCommand(disableableHandler), telegram.Filter{Func: helpers.IsAdmin})
	client.On("command:disabled", handlers.HandleCommand(disabledHandler), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("command:config", handlers.HandleCommand(configHandler), telegram.Filter{Group: true, Func: helpers.IsAdmin})
	client.On("callback:^config", configCallback, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:^languageMenu", languageMenuCallback, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:^setLang", languageSetCallback, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:^mediaConfig", mediaConfigCallback, telegram.Filter{Func: helpers.IsAdmin})
	client.On("callback:^ieConfig", explainConfigCallback, telegram.Filter{Func: helpers.IsAdmin})
}
