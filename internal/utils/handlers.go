package utils

import (
	"fmt"

	"github.com/ruizlenato/smudgelord/internal/database"
)

var DisableableCommands []string

func CheckDisabledCommand(command string, chatID int64) bool {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? AND chat_id = ? LIMIT 1);"
	err := database.DB.QueryRow(query, command, chatID).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking command: %v\n", err)
		return false
	}
	return exists
}
