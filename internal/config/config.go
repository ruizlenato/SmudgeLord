package config

import (
	"log"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	TelegramAPIID    int32
	TelegramAPIHash  string
	TelegramBotToken string
	LogLevel         slog.Leveler
	DatabaseFile     string
	OwnerID          int64
	LogChannelID     int64
	Socks5Proxy      string
	LastFMAPIKey     string
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

	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr == "" {
		logLevelStr = "ERROR"
	}
	LogLevel = parseLogLevel(logLevelStr)

	DatabaseFile = os.Getenv("DATABASE_FILE")

	OwnerID, _ = strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
	if OwnerID == 0 {
		log.Fatalf(`You need to set the "OWNER_ID" in the .env file!`)
	}

	LogChannelID, _ = strconv.ParseInt(os.Getenv("CHANNEL_LOG_ID"), 10, 64)
	if LogChannelID == 0 {
		log.Fatalf(`You need to set the "CHANNEL_LOG_ID" in the .env file!`)
	}

	Socks5Proxy = os.Getenv("SOCKS5_PROXY")

	LastFMAPIKey = os.Getenv("LASTFM_API_KEY")
	if LastFMAPIKey == "" {
		log.Fatalf(`You need to set the "LASTFM_API_KEY" in the .env file!`)
	}
}

func parseLogLevel(level string) slog.Leveler {
	levels := map[string]slog.Level{
		"ERROR":   slog.LevelError,
		"INFO":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"WARNING": slog.LevelWarn,
		"WARN":    slog.LevelWarn,
	}

	l, ok := levels[level]
	if !ok {
		l = slog.LevelError
	}

	return l
}
