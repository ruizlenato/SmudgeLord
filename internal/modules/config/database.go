package config

import (
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func insertDisabledCommand(chatID int64, command string) error {
	_, err := database.DB.Exec("INSERT OR IGNORE INTO commandsDisabled (chat_id, command) VALUES (?, ?);", chatID, command)
	if err != nil {
		return err
	}
	utils.InvalidateDisabledCommandsCache(chatID)
	return nil
}

func deleteDisabledCommand(chatID int64, command string) error {
	_, err := database.DB.Exec("DELETE FROM commandsDisabled WHERE chat_id = ? AND command = ?;", chatID, command)
	if err != nil {
		return err
	}
	utils.InvalidateDisabledCommandsCache(chatID)
	return nil
}

func getDisabledCommands(chatID int64) ([]string, error) {
	rows, err := database.DB.Query("SELECT command FROM commandsDisabled WHERE chat_id = ?;", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commands := make([]string, 0)
	for rows.Next() {
		var command string
		if err := rows.Scan(&command); err != nil {
			continue
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

func getMediaConfig(chatID int64) (bool, bool, bool, error) {
	var mediasCaption, mediasAuto, mediasErrors bool
	err := database.DB.QueryRow("SELECT mediasCaption, mediasAuto, mediasErrors FROM chats WHERE id = ?;", chatID).Scan(&mediasCaption, &mediasAuto, &mediasErrors)
	return mediasCaption, mediasAuto, mediasErrors, err
}
