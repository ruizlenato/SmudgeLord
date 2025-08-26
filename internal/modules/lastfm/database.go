package lastfm

import (
	"log/slog"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
)

func setLastFMUsername(sender *telegram.UserObj, lastFMUsername string) error {
	if !database.ChatExists(sender.ID, telegram.EntityUser, sender.Username) {
		if err := database.SaveChat(sender.ID, telegram.EntityUser, sender); err != nil {
			slog.Error(
				"Error saving chat",
				"error", err.Error(),
			)
			return err
		}
	}
	_, err := database.DB.Exec("UPDATE users SET lastfm_username = ? WHERE id = ?;", lastFMUsername, sender.ID)
	if err != nil {
		return err
	}
	return nil
}

func getUserLastFMUsername(userID int64) (string, error) {
	var lastFMUsername string
	err := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", userID).Scan(&lastFMUsername)
	return lastFMUsername, err
}
