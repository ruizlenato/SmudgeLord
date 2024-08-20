package database

import (
	"database/sql"
	"fmt"
	"log"
	"slices"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
)

var DB *sql.DB

var AvailableLocales []string

func Open(databaseFile string) error {
	db, err := sql.Open("sqlite3", databaseFile+"?_journal_mode=WAL")
	if err != nil {
		return err
	}

	// Check if journal_mode is set to WAL
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		db.Close()
		return err
	}

	// Set the global DB variable to the opened database.
	DB = db

	return nil
}

func CreateTables() error {
	query := `
        CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
            language TEXT DEFAULT 'en-us',
			username TEXT,
			lastfm_username TEXT
        );
		CREATE TABLE IF NOT EXISTS groups (
            id INTEGER PRIMARY KEY,
            language TEXT DEFAULT 'en-us',
			mediasAuto BOOLEAN DEFAULT 1,
			mediasCaption BOOLEAN DEFAULT 1,
			lastFMCommands BOOLEAN DEFAULT 1
        );
		CREATE TABLE IF NOT EXISTS afk (
			id INTEGER PRIMARY KEY,
			reason TEXT,
			time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
    `
	_, err := DB.Exec(query)
	return err
}

func Close() {
	fmt.Println("Database closed")
	if DB != nil {
		DB.Close()
	}
}

func SaveUsers(bot *telego.Bot, update telego.Update, next telegohandler.Handler) {
	message := update.Message
	var username string

	if message == nil {
		if update.CallbackQuery == nil {
			return
		}
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	if message.SenderChat != nil {
		return
	}

	if message.From.ID != message.Chat.ID {
		query := "INSERT OR IGNORE INTO groups (id) VALUES (?);"
		_, err := DB.Exec(query, message.Chat.ID)
		if err != nil {
			log.Print("[database/SaveUsers] Error inserting group: ", err)
		}
	}

	query := `
		INSERT INTO users (id, language, username)
    	VALUES (?, ?, ?)
    	ON CONFLICT(id) DO UPDATE SET 
			username = excluded.username;
	`

	if message.From.Username != "" {
		username = "@" + message.From.Username
	}

	lang := message.From.LanguageCode
	if !slices.Contains(AvailableLocales, lang) {
		lang = "en-us"
	}
	_, err := DB.Exec(query, message.From.ID, lang, username)
	if err != nil {
		log.Print("[database/SaveUsers] Error upserting user: ", err)
	}

	next(bot, update)
}
