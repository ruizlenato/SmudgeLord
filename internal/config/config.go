package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	TelegramToken string
	LogLevel      slog.Leveler
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
		slog.Error("Error loading .env file",
			"error", err.Error())
	}

	TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	if TelegramToken == "" {
		slog.Error(`You need to set the "TELEGRAM_TOKEN" in the .env file!`)
		os.Exit(1)

	}

	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr == "" {
		logLevelStr = "ERROR"
	}
	LogLevel = parseLogLevel(logLevelStr)

	LastFMKey = os.Getenv("LASTFM_API_KEY")
	if LastFMKey == "" {
		slog.Error(`You need to set the "LASTFM_API_KEY" in the .env file!`)
		os.Exit(1)
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
		slog.Error(`You need to set the "OWNER_ID" in the .env file!`)
		os.Exit(1)

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
