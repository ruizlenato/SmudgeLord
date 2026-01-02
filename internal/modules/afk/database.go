package afk

import (
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/ruizlenato/smudgelord/internal/database"
)

func userIsAway(userID int64) bool {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM afk WHERE id = ?)", userID).Scan(&exists)
	if err != nil {
		slog.Error("Couldn't check AFK",
			"UserID", userID,
			"Error", err.Error())
	}
	return exists
}

func getUserAway(userID int64) (string, time.Duration, error) {
	row := database.DB.QueryRow("SELECT reason, time FROM afk WHERE id = ?", userID)

	var reason string
	var afkTime time.Time
	err := row.Scan(&reason, &afkTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, nil
		}
		return "", 0, errors.New("Error getting AFK info: " + err.Error())
	}

	return reason, time.Since(afkTime), nil
}

func setUserAway(sender *models.User, reason string, time time.Time) error {
	query := `
        INSERT INTO afk (id, username, reason, time) VALUES (?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            username = excluded.username,
            reason = excluded.reason,
            time = excluded.time
    `

	username := database.FormatUsername(sender.Username)

	_, err := database.DB.Exec(query, sender.ID, username, reason, time)
	if err != nil {
		return errors.New("Error setting AFK: " + err.Error())
	}
	return nil
}

func unsetUserAway(userID int64) error {
	statement, err := database.DB.Prepare("DELETE FROM afk WHERE id = ?")
	if err != nil {
		return errors.New("Error preparing unsetAFK statement: " + err.Error())
	}
	defer statement.Close()

	_, err = statement.Exec(userID)
	if err != nil {
		return errors.New("Error unsetting AFK: " + err.Error())
	}
	return nil
}

func getIDFromUsername(username string) (int64, error) {
	var id int64
	row := database.DB.QueryRow("SELECT id FROM afk WHERE username = ?", username)

	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, errors.New("Error getting user ID: " + err.Error())
	}

	return id, nil
}
