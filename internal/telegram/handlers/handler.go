package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
)

var DisableableCommands []string

func HandleCommand(handler func(m *telegram.NewMessage) error) func(m *telegram.NewMessage) error {
	return func(m *telegram.NewMessage) error {
		command := strings.Replace(m.GetCommand(), "/", "", 1)
		if CheckDisabledCommand(m, command) {
			return nil
		}
		if err := database.SaveUsers(m); err != nil {
			slog.Error(
				"Could not save user",
				"error", err.Error(),
			)
		}
		if err := handler(m); err != nil {
			if strings.Contains(err.Error(), "CHAT_SEND_PLAIN_FORBIDDEN") ||
				strings.Contains(err.Error(), "CHAT_WRITE_FORBIDDEN") {
				return nil
			}
			return err
		}
		return nil
	}
}

func CheckDisabledCommand(message *telegram.NewMessage, command string) bool {
	var exists bool
	if message.IsPrivate() {
		return false
	}

	query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? AND chat_id = ? LIMIT 1);"
	if err := database.DB.QueryRow(query, command, message.Chat.ID).Scan(&exists); err != nil {
		fmt.Printf("Error checking command: %v\n", err)
		return false
	}
	return exists
}
