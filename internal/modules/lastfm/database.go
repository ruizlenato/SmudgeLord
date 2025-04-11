package lastfm

import (
	"github.com/ruizlenato/smudgelord/internal/database"
)

func setLastFMUsername(userID int64, lastFMUsername string) error {
	_, err := database.DB.Exec("UPDATE users SET lastfm_username = ? WHERE id = ?;", lastFMUsername, userID)
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
