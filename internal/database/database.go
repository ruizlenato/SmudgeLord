package database

import (
	"context"
	"database/sql"
	"fmt"
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
		err := DB.Close()
		if err == nil {
			fmt.Println("Database closed")
		}
	}
}

func SaveUsers(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		var chatID int64
		var chatType models.ChatType
		var sender *models.User

		if update.Message != nil {
			chatID = update.Message.Chat.ID
			chatType = update.Message.Chat.Type
			sender = update.Message.From
		} else if update.CallbackQuery != nil {
			message := update.CallbackQuery.Message.Message
			chatID = message.Chat.ID
			chatType = message.Chat.Type
			sender = message.From
		} else if update.InlineQuery != nil {
			chatID = update.InlineQuery.From.ID
			chatType = models.ChatTypePrivate
			sender = update.InlineQuery.From
		} else {
			next(ctx, b, update)
			return
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

			}
		}

		next(ctx, b, update)
	}
}

func ChatExists(chatID int64, chatType models.ChatType, senderUsername string) bool {
	if chatID == 0 {
		return true
	}

	switch chatType {
	case models.ChatTypePrivate:
		row := DB.QueryRow("SELECT username FROM users WHERE id = ?", chatID)

		var savedUsername sql.NullString
		err := row.Scan(&savedUsername)
		if err != nil {
			return false
		}
		currentUsername := FormatUsername(senderUsername)
		return savedUsername.String == currentUsername
	case models.ChatTypeGroup, models.ChatTypeSupergroup:
		row := DB.QueryRow("SELECT id FROM chats WHERE id = ?", chatID)
		var id int64
		err := row.Scan(&id)
		return err == nil
	default:
		return true
	}
}

func SaveChat(chatID int64, chatType models.ChatType, sender *models.User) error {
	if chatID == 0 {
		return nil
	}

	switch chatType {
	case models.ChatTypePrivate:
		query := `
			INSERT INTO users (id, language, username) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET username = excluded.username
        `

		username := FormatUsername(sender.Username)
		language := getValidLanguage(sender.LanguageCode)

		_, err := DB.Exec(query, chatID, language, username)
		return err
	case models.ChatTypeGroup, models.ChatTypeSupergroup:
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
