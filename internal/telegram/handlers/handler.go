package handlers

import (
	"fmt"
	"log"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
)

var DisableableCommands []string

func HandleCommand(handler func(m *telegram.NewMessage) error) func(m *telegram.NewMessage) error {
	return func(m *telegram.NewMessage) error {
		if CheckDisabledCommand(strings.Split(m.Text(), " ")[0]) {
			return nil
		}
		err := database.SaveUsers(m)
		if err != nil {
			log.Printf("Error saving user: %v\n", err)
		}
		return handler(m)
	}
}

func CheckDisabledCommand(command string) bool {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? LIMIT 1);"
	err := database.DB.QueryRow(query, command).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking command: %v\n", err)
		return false
	}
	return exists
}
