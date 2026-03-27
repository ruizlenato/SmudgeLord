package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
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

func CreateBackupFile() (string, error) {
	if DB == nil {
		return "", fmt.Errorf("database is not initialized")
	}

	backupPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("smudgelord-backup-%s.db", time.Now().UTC().Format("20060102-150405")),
	)

	if _, err := DB.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return "", fmt.Errorf("wal checkpoint: %w", err)
	}

	escapedPath := strings.ReplaceAll(backupPath, "'", "''")
	if _, err := DB.Exec(fmt.Sprintf("VACUUM INTO '%s';", escapedPath)); err != nil {
		return "", fmt.Errorf("vacuum into backup file: %w", err)
	}

	return backupPath, nil
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
		CREATE TABLE IF NOT EXISTS sticker_packs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			pack_name TEXT NOT NULL,
			is_default BOOLEAN DEFAULT 0,
			UNIQUE(user_id, pack_name)
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

func SaveUsers(_ *gotgbot.Bot, ctx *ext.Context) error {
	if ctx == nil || ctx.Update == nil {
		return nil
	}

	var chatID int64
	var chatType *string
	var sender *gotgbot.User

	if update := ctx.Update; update.Message != nil {
		chatID = update.Message.Chat.Id
		tmpChatType := update.Message.Chat.Type
		chatType = &tmpChatType
		sender = update.Message.From
	} else if update.CallbackQuery != nil {
		tmpChatType := gotgbot.ChatTypePrivate
		chatType = &tmpChatType
		if update.CallbackQuery.Message != nil {
			chat := update.CallbackQuery.Message.GetChat()
			chatID = chat.Id
			tmpChatType = chat.Type
			chatType = &tmpChatType
		}
		sender = &update.CallbackQuery.From
	} else if update.InlineQuery != nil {
		tmpChatType := gotgbot.ChatTypePrivate
		chatType = &tmpChatType
		sender = &update.InlineQuery.From
		chatID = sender.Id
	} else {
		return nil
	}

	if sender == nil {
		return nil
	}

	if !ChatExists(chatID, chatType, sender.Username) {
		slog.Debug(
			"Chat does not exist in database, saving...",
			"ChatID", chatID,
			"ChatType", chatType,
		)
		if err := SaveChat(chatID, chatType, sender); err != nil {
			slog.Error("Error saving chat", "error", err.Error())
		}
	}

	return nil
}

func ChatExists(chatID int64, chatType *string, senderUsername string) bool {
	if chatType == nil {
		return true
	}

	if chatID == 0 && *chatType != gotgbot.ChatTypePrivate {
		return true
	}

	switch *chatType {
	case gotgbot.ChatTypePrivate:
		row := DB.QueryRow("SELECT username FROM users WHERE id = ?", chatID)
		var savedUsername sql.NullString
		err := row.Scan(&savedUsername)
		if err != nil {
			return false
		}
		currentUsername := FormatUsername(senderUsername)
		return savedUsername.String == currentUsername
	case gotgbot.ChatTypeGroup, gotgbot.ChatTypeSupergroup:
		row := DB.QueryRow("SELECT id FROM chats WHERE id = ?", chatID)
		var id int64
		err := row.Scan(&id)
		return err == nil
	default:
		return true
	}
}

func SaveChat(chatID int64, chatType *string, sender *gotgbot.User) error {
	if chatID == 0 {
		return nil
	}

	if chatType == nil || sender == nil {
		return nil
	}

	switch *chatType {
	case gotgbot.ChatTypePrivate:
		query := `
			INSERT INTO users (id, language, username) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET username = excluded.username
        `

		username := FormatUsername(sender.Username)
		language := getValidLanguage(sender.LanguageCode)

		_, err := DB.Exec(query, chatID, language, username)
		return err
	case gotgbot.ChatTypeGroup, gotgbot.ChatTypeSupergroup:
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
