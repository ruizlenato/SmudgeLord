package afk

import (
	"database/sql"
	"errors"
	"time"

	"github.com/ruizlenato/smudgelord/internal/database"
)

func userIsAway(user_id int64) (bool, error) {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM afk WHERE id = ?)", user_id).Scan(&exists)
	if err != nil {
		return false, errors.New("Error preparing userIsAway statement: " + err.Error())
	}
	return exists, nil
}

func setUserAway(user_id int64, reason string, time time.Time) error {
	stmt, err := database.DB.Prepare("INSERT OR IGNORE INTO afk (id, reason, time) VALUES (?, ?, ?)")
	if err != nil {
		return errors.New("Error preparing setAFK statement: " + err.Error())
	}
	defer stmt.Close()

	_, err = stmt.Exec(user_id, reason, time)
	if err != nil {
		return errors.New("Error setting AFK: " + err.Error())
	}
	return nil
}

func getUserAway(user_id int64) (string, time.Duration, error) {
	row := database.DB.QueryRow("SELECT reason, time FROM afk WHERE id = ?", user_id)

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

func unsetUserAway(user_id int64) error {
	statement, err := database.DB.Prepare("DELETE FROM afk WHERE id = ?")
	if err != nil {
		return errors.New("Error preparing unsetAFK statement: " + err.Error())
	}
	defer statement.Close()

	_, err = statement.Exec(user_id)
	if err != nil {
		return errors.New("Error unsetting AFK: " + err.Error())
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
		return 0, errors.New("Error getting user ID: " + err.Error())
	}

	return id, nil
}
