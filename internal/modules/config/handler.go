package config

import (
	"fmt"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
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
		_, err := message.Reply("command not disableable")
		return err
	}

	if handlers.CheckDisabledCommand(command) {
		_, err := message.Reply(fmt.Sprintf(i18n("config.cmdAlreadyDisabled"), command))
		return err
	}

	query := "INSERT INTO commandsDisabled (command) VALUES (?);"
	_, err := database.DB.Exec(query, command)
	if err != nil {
		fmt.Printf("Error inserting command: %v\n", err)
		return err
	}
	_, err = message.Reply(fmt.Sprintf(i18n("config.cmdDisabled"), command))

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
		_, err := message.Reply(fmt.Sprintf(i18n("config.cmdAlreadyEnabled"), command))
		return err
	}

	query := "DELETE FROM commandsDisabled WHERE command = ?;"
	_, err := database.DB.Exec(query, command)
	if err != nil {
		fmt.Printf("Error deleting command: %v\n", err)
		return err
	}
	_, err = message.Reply(fmt.Sprintf(i18n("config.cmdEnabled"), command))

	return err
}

func handlerDisableableCommands(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	text := i18n("config.cmdDisableables")
	for _, command := range handlers.DisableableCommands {
		text += "\n- " + command
	}
	_, err := message.Reply(text)

	return err
}

func Load(client *telegram.Client) {
	client.On("command:disable", handlers.HanndleCommand(handlerDisable))
	client.On("command:enable", handlers.HanndleCommand(handlerEnable))
	client.On("command:disableable", handlers.HanndleCommand(handlerDisableableCommands))
}
