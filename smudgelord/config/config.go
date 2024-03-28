package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

var (
	TelegramToken string
	LastFMKey     string
	DatabaseFile  string
	WebhookURL    string
	SOCKS5URL     string
)

// init initializes the config variables.
func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if TelegramToken == "" {
		log.Fatalf(`You need to set the "TELEGRAM_TOKEN" in the .env file!`)
	}

	LastFMKey = os.Getenv("LASTFM_API_KEY")

	DatabaseFile = os.Getenv("DATABASE_FILE")
	if DatabaseFile == "" {
		DatabaseFile = filepath.Join(".", "smudgelord", "database", "database.sql")
	}

	WebhookURL = os.Getenv("WEBHOOK_URL")
	SOCKS5URL = os.Getenv("SOCKS5URL")
}
