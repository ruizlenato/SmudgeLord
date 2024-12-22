package database

import (
	"database/sql"
	"fmt"
	"log/slog"
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

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		db.Close()
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
		CREATE TABLE IF NOT EXISTS groups (
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
			next(bot, update)
			return
		}

		switch msg := update.CallbackQuery.Message.(type) {
		case *telego.Message:
			message = msg
		default:
			next(bot, update)
			return
		}
	}

	if message.SenderChat != nil {
		return
	}

	if message.From.ID != message.Chat.ID {
		query := "INSERT OR IGNORE INTO groups (id) VALUES (?);"
		_, err := DB.Exec(query, message.Chat.ID)
		if err != nil {
			slog.Error("Couldn't insert group", "ChatID", message.Chat.ID, "Error", err.Error())
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
		slog.Error("Couldn't insert user", "UserID", message.From.ID, "Username", username, "Error", err.Error())
	}

	next(bot, update)
}
