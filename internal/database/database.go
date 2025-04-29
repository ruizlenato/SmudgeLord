package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	_ "github.com/mattn/go-sqlite3"
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
	if DB != nil {
		err := DB.Close()
		if err == nil {
			fmt.Println("Database closed")
		}
	}
}

func SaveUsers(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		message := update.Message
		var username string

		if update.Message == nil {
			if update.CallbackQuery == nil {
				next(ctx, b, update)
				return
			}

			message = update.CallbackQuery.Message.Message

			if update.CallbackQuery.Message.Type == 1 || message == nil {
				next(ctx, b, update)
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
				slog.Error("Couldn't insert group",
					"ChatID", message.Chat.ID,
					"Error", err.Error())
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
			slog.Error("Couldn't insert user",
				"UserID", message.From.ID,
				"Username", username,
				"Error", err.Error())
		}

		if update.Message != nil {
			log.Printf("%d say: %s", update.Message.From.ID, update.Message.Text)
		}

		next(ctx, b, update)
	}
}
