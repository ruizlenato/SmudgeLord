package database

import (
	"database/sql"
	"fmt"
	"log"
	"slices"

	"github.com/amarnathcjd/gogram/telegram"
	_ "github.com/mattn/go-sqlite3"
)

// DB is a global variable representing the SQLite database connection.
// It is initialized using the Open function and can be used throughout the application to interact with the database.
var DB *sql.DB

// AvailableLocales is a global variable storing a list of available language locales.
// It is used to validate and set user language preferences in the database.
var AvailableLocales []string

// Open initializes a SQLite database connection using the provided database file.
// It opens the SQLite database with Write-Ahead Logging (WAL) journal mode for better concurrency and performance.
//
// Parameters:
// - databaseFile: A string representing the path to the SQLite database file.
//
// Returns:
// - An error if there is an issue opening or configuring the database; otherwise, it returns nil.
//
// Example Usage:
//
//	err := Open("example.db")
//	if err != nil {
//	    log.Fatal("Error opening database:", err)
//	}
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

// CreateTables creates the necessary tables in the database if they do not already exist.
// It defines the schema for the 'users' and 'groups' tables, including columns for user and group information.
//
// Returns:
// - An error if there is an issue executing the SQL queries; otherwise, it returns nil.
//
// Example Usage:
//
//	err := CreateTables()
//	if err != nil {
//	    log.Fatal("Error creating tables:", err)
//	}
func CreateTables() error {
	query := `
        CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
            language TEXT DEFAULT 'en-us',
			username TEXT,
			lastfm_username TEXT
        );
		CREATE TABLE IF NOT EXISTS chats (
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

// Close closes the database connection.
// It prints a message indicating that the database is closed and ensures that the database connection is properly closed.
//
// Example Usage:
//
//	Close()
func Close() {
	fmt.Println("[!] — Database closed")
	if DB != nil {
		DB.Close()
	}
}

// SaveUsers inserts information about users and groups in the database based on the provided update.
// It extracts message information from the update and performs the following actions:
//   - If the message is sent by the sender's chat (e.g., channels or anonymous users), the function returns without further processing.
//   - If the message is from a group, it inserts the group's ID into the 'groups' table.
//   - Inserts user information into the 'users' table, including the user's ID and language code.
//   - If the user's language code is not in the list of available locales, it defaults to "en-us".
//
// Note:
// - This function is intended to be used as a middleware in a Telego handler chain.
// - Ensure that the DB variable is correctly initialized before calling this function.
func SaveUsers(message *telegram.NewMessage) error {
	var username string

	if message.Sender.ID == message.Client.Me().ID {
		return nil
	}

	if chatID := message.ChatID(); chatID != message.Sender.ID {
		query := "INSERT OR IGNORE INTO chats (id) VALUES (?);"
		_, err := DB.Exec(query, chatID)
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
	if message.Sender.Username != "" {
		username = "@" + message.Sender.Username
	}

	lang := message.Sender.LangCode
	if !slices.Contains(AvailableLocales, lang) {
		lang = "en-us"
	}
	_, err := DB.Exec(query, message.Sender.ID, lang, username)
	if err != nil {
		log.Print("[database/SaveUsers] Error upserting user: ", err)
	}

	return nil
}
