package database

import (
	"database/sql"
	"fmt"
	"slices"

	"github.com/amarnathcjd/gogram/telegram"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/exp/slog"
)

var DB *sql.DB

var AvailableLocales []string

func Open(databaseFile string) error {
	db, err := sql.Open("sqlite3", databaseFile+"?_journal_mode=WAL")
	if err != nil {
		return err
	}

	DB = db
	return nil
}

func CreateTables() error {
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			language TEXT NOT NULL DEFAULT 'en-us',
			username TEXT,
			lastfm_username TEXT
		);
		CREATE TABLE IF NOT EXISTS chats (
			id INTEGER PRIMARY KEY,
			language TEXT DEFAULT 'en-us',
			mediasAuto BOOLEAN DEFAULT 1,
			mediasCaption BOOLEAN DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS afk (
			id INTEGER PRIMARY KEY,
			username TEXT,
			reason TEXT,
			time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS commandsDisabled (
			chat_id INTEGER,
			command TEXT NOT NULL,
			PRIMARY KEY (chat_id, command)
		);
	`
	_, err := DB.Exec(query)
	return err
}

func Close() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			slog.Error(
				"Error closing database",
				"error", err.Error(),
			)
		} else {
			fmt.Println("[!] — Database closed")
		}
	}
}

func SaveUsers(update any) error {
	var chatID int64
	var chatType string
	var sender *telegram.UserObj

	switch u := update.(type) {
	case *telegram.NewMessage:
		chatID = u.ChatID()
		chatType = u.ChatType()
		sender = u.Sender
	default:
		return nil
	}

	if !ChatExists(chatID, chatType, sender.Username) {
		slog.Debug(
			"Chat does not exist in database, saving...",
			"ChatID", chatID,
			"ChatType", chatType,
		)
		if err := SaveChat(chatID, chatType, sender); err != nil {
			slog.Error(
				"Error saving chat",
				"error", err.Error(),
			)
			return err
		}
	}

	return nil
}

func ChatExists(chatID int64, chatType, senderUsername string) bool {
	if chatID == 0 {
		return true
	}

	switch chatType {
	case "user":
		row := DB.QueryRow("SELECT username FROM users WHERE id = ?", chatID)

		var savedUsername sql.NullString
		err := row.Scan(&savedUsername)
		if err != nil {
			return false
		}
		currentUsername := FormatUsername(senderUsername)
		return savedUsername.String == currentUsername
	case "chat":
		row := DB.QueryRow("SELECT id FROM chats WHERE id = ?", chatID)
		var id int64
		err := row.Scan(&id)
		return err == nil
	default:
		return true
	}
}

func SaveChat(chatID int64, chatType string, sender *telegram.UserObj) error {
	if chatID == 0 {
		return nil
	}

	switch chatType {
	case "user":
		query := `
			INSERT INTO users (id, language, username) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET username = excluded.username
        `

		username := FormatUsername(sender.Username)
		language := getValidLanguage(sender.LangCode)

		_, err := DB.Exec(query, chatID, language, username)
		return err
	case "chat":
		query := "INSERT OR IGNORE INTO chats (id) VALUES (?);"
		_, err := DB.Exec(query, chatID)
		return err
	default:
		return nil
	}

}

func FormatUsername(username string) string {
	if username == "" {
		return ""
	}
	return "@" + username
}

func getValidLanguage(langCode string) string {
	if slices.Contains(AvailableLocales, langCode) {
		return langCode
	}
	return "en-us"
}
