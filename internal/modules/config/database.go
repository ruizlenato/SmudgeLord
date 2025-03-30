package config

import (
	"github.com/ruizlenato/smudgelord/internal/database"
)

func getDisabledCommands(chatID int64) ([]string, error) {
	rows, err := database.DB.Query("SELECT command FROM commandsDisabled WHERE chat_id = ?", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []string
	for rows.Next() {
		var command string
		if err := rows.Scan(&command); err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	return commands, nil
}

func insertDisabledCommand(chatID int64, command string) error {
	_, err := database.DB.Exec("INSERT INTO commandsDisabled (chat_id, command) VALUES (?, ?);", chatID, command)
	return err
}

func deleteDisabledCommand(command string) error {
	_, err := database.DB.Exec("DELETE FROM commandsDisabled WHERE command = ?;", command)
	return err
}
