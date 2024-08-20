package database

import (
	"database/sql"
	"fmt"
	"log"
	"slices"

	"github.com/amarnathcjd/gogram/telegram"
	_ "github.com/mattn/go-sqlite3"
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
			chat_id INTEGER PRIMARY KEY,
			command TEXT NOT NULL
		);
	`
	_, err := DB.Exec(query)
	return err
}

func Close() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		} else {
			fmt.Println("[!] — Database closed")
		}
	}
}

func SaveUsers(message *telegram.NewMessage) error {
	if message.Sender.ID == message.Client.Me().ID {
		return nil
	}

	if err := saveChat(message.ChatID()); err != nil {
		log.Printf("[database/SaveUsers] Error inserting group: %v", err)
	}

	if err := saveUser(message); err != nil {
		log.Printf("[database/SaveUsers] Error upserting user: %v", err)
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

func saveUser(message *telegram.NewMessage) error {
	query := `
		INSERT INTO users (id, language, username)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			username = excluded.username;
	`

	username := ""
	if message.Sender.Username != "" {
		username = "@" + message.Sender.Username
	}

	lang := message.Sender.LangCode
	if !slices.Contains(AvailableLocales, lang) {
		lang = "en-us"
	}

	_, err := DB.Exec(query, message.Sender.ID, lang, username)
	return err
}
