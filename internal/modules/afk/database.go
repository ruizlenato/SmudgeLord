package afk

import (
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
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

type awayInfo struct {
	firstName string
	reason    string
	duration  time.Duration
	mediaType string
	mediaID   string
}

func getUserAway(userID int64) (awayInfo, error) {
	row := database.DB.QueryRow("SELECT first_name, reason, time, media FROM afk WHERE id = ?", userID)

	var info awayInfo
	var firstName sql.NullString
	var media sql.NullString
	var afkTime time.Time

	err := row.Scan(&firstName, &info.reason, &afkTime, &media)
	if err != nil {
		if err == sql.ErrNoRows {
			return info, nil
		}
		return info, errors.New("Error getting AFK info: " + err.Error())
	}

	info.firstName = firstName.String
	info.duration = time.Since(afkTime)
	if mediaType, mediaID, found := strings.Cut(media.String, ":"); found && mediaID != "" {
		info.mediaType, info.mediaID = mediaType, mediaID
	}

	return info, nil
}

func setUserAway(sender *gotgbot.User, reason, media string, time time.Time) error {
	query := `
	INSERT INTO afk (id, username, first_name, reason, media, time) VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		username = excluded.username,
		first_name = excluded.first_name,
		reason = excluded.reason,
		media = excluded.media,
		time = excluded.time
	`

	if sender == nil {
		return nil
	}

	username := sender.Username
	if username != "" {
		username = "@" + strings.TrimPrefix(username, "@")
	}

	_, err := database.DB.Exec(query, sender.Id, username, sender.FirstName, reason, media, time)
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
	row := database.DB.QueryRow("SELECT id FROM afk WHERE lower(username) = lower(?)", username)

	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, errors.New("Error getting user ID: " + err.Error())
	}

	return id, nil
}
