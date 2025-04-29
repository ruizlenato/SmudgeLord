package afk

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/ruizlenato/smudgelord/internal/database"
)

type AFKError struct {
	msg string
}

func (e *AFKError) Error() string {
	return e.msg
}

func user_is_away(user_id int64) bool {
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM afk WHERE id = ?", user_id).Scan(&count)
	if err != nil {
		slog.Error("Couldn't check AFK",
			"UserID", user_id,
			"Error", err.Error())
	}
	return count > 0
}

func get_user_away(user_id int64) (string, time.Duration, error) {
	row := database.DB.QueryRow("SELECT reason, time FROM afk WHERE id = ?", user_id)

	var reason string
	var afkTime time.Time
	err := row.Scan(&reason, &afkTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, nil // User not found
		}
		return "", 0, &AFKError{fmt.Sprintf("Error getting AFK info: %v", err)}
	}

	return reason, time.Since(afkTime), nil
}

func set_user_away(user_id int64, reason string, time time.Time) error {
	stmt, err := database.DB.Prepare("INSERT OR IGNORE INTO afk (id, reason, time) VALUES (?, ?, ?)")
	if err != nil {
		return &AFKError{fmt.Sprintf("Error preparing setAFK statement: %v", err)}
	}
	defer stmt.Close()

	_, err = stmt.Exec(user_id, reason, time)
	if err != nil {
		return &AFKError{fmt.Sprintf("Error setting AFK: %v", err)}
	}
	return nil
}

func unset_user_away(user_id int64) error {
	stmt, err := database.DB.Prepare("DELETE FROM afk WHERE id = ?")
	if err != nil {
		return &AFKError{fmt.Sprintf("Error preparing unsetAFK statement: %v", err)}
	}
	defer stmt.Close()

	_, err = stmt.Exec(user_id)
	if err != nil {
		return &AFKError{fmt.Sprintf("Error unsetting AFK: %v", err)}
	}
	return nil
}

func getIDFromUsername(username string) (int64, error) {
	var id int64
	row := database.DB.QueryRow("SELECT id FROM users WHERE username = ?", username)

	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, &AFKError{fmt.Sprintf("Error getting user ID: %v", err)}
	}

	return id, nil
}
