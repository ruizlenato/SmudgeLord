package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	TelegramToken string
	LastFMKey     string
	DatabaseFile  string
	BotAPIURL     string
	WebhookURL    string
	WebhookPort   int
	Socks5Proxy   string
	OwnerID       int64
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if TelegramToken == "" {
		log.Fatalf(`You need to set the "TELEGRAM_TOKEN" in the .env file!`)
	}

	LastFMKey = os.Getenv("LASTFM_API_KEY")
	if LastFMKey == "" {
		log.Fatalf(`You need to set the "LASTFM_API_KEY" in the .env file!`)
	}

	DatabaseFile = os.Getenv("DATABASE_FILE")

	WebhookPort, _ = strconv.Atoi(os.Getenv("WEBHOOK_PORT"))
	if WebhookPort == 0 {
		WebhookPort = 8080
	}

	WebhookURL = os.Getenv("WEBHOOK_URL")
	BotAPIURL = os.Getenv("BOTAPI_URL")

	Socks5Proxy = os.Getenv("SOCKS5_PROXY")

	OwnerID, _ = strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
	if OwnerID == 0 {
		log.Fatalf(`You need to set the "OWNER_ID" in the .env file!`)
	}
}
