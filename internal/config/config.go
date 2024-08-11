package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	TelegramAPIID    int32
	TelegramAPIHash  string
	TelegramBotToken string
	DatabaseFile     string
	OWNERID          int64
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	tempAPIID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_API_ID"), 10, 32)
	TelegramAPIID = int32(tempAPIID)
	if TelegramAPIID == 0 {
		log.Fatalf(`You need to set the "TELEGRAM_API_ID" in the .env file!`)
	}

	TelegramAPIHash = os.Getenv("TELEGRAM_API_HASH")
	if TelegramAPIHash == "" {
		log.Fatalf(`You need to set the "TELEGRAM_API_HASH" in the .env file!`)
	}

	TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if TelegramBotToken == "" {
		log.Fatalf(`You need to set the "TELEGRAM_BOT_TOKEN" in the .env file!`)
	}

	DatabaseFile = os.Getenv("DATABASE_FILE")

	OWNERID, _ = strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
	if OWNERID == 0 {
		log.Fatalf(`You need to set the "OWNER_ID" in the .env file!`)
	}
}
