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
	var sender *telegram.UserObj
	var client *telegram.Client
	var chatID int64

	switch u := update.(type) {
	case *telegram.NewMessage:
		sender = u.Sender
		client = u.Client
		chatID = u.ChatID()
	case *telegram.InlineQuery:
		sender = u.Sender
		client = u.Client
	case *telegram.InlineSend:
		sender = u.Sender
		client = u.Client
	default:
		return nil
	}

	if sender.ID == client.Me().ID {
		return nil
	}

	if chatID != sender.ID {
		if err := saveChat(chatID); err != nil {
			slog.Error(
				"Could not save chat",
				"ChatID", chatID,
				"error", err.Error(),
			)
		}
	}

	if err := saveUser(sender); err != nil {
		slog.Error(
			"Could not save user",
			"UserID", sender.ID,
			"error", err.Error(),
		)
	}

	return nil
}

func saveChat(chatID int64) error {
	if chatID == 0 {
		return nil
	}

	query := "INSERT OR IGNORE INTO chats (id) VALUES (?);"
	_, err := DB.Exec(query, chatID)
	return err
}

func saveUser(sender *telegram.UserObj) error {
	query := `
		INSERT INTO users (id, language, username)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			username = excluded.username;
	`

	username := ""
	if sender.Username != "" {
		username = "@" + sender.Username
	}

	lang := sender.LangCode
	if !slices.Contains(AvailableLocales, lang) {
		lang = "en-us"
	}

	_, err := DB.Exec(query, sender.ID, lang, username)
	return err
}
