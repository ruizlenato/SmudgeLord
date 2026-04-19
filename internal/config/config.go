package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

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
	Socks5Proxies []string
	OwnerID       int64
	LogChannelID  int64
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
	Socks5Proxies = parseProxyList(Socks5Proxy)

	LogChannelID, _ = strconv.ParseInt(os.Getenv("CHANNEL_LOG_ID"), 10, 64)
	if LogChannelID == 0 {
		slog.Error(`You need to set the "CHANNEL_LOG_ID" in the .env file!`)
		os.Exit(1)
	}

	OwnerID, _ = strconv.ParseInt(os.Getenv("OWNER_ID"), 10, 64)
	if OwnerID == 0 {
		slog.Error(`You need to set the "OWNER_ID" in the .env file!`)
		os.Exit(1)
	}
}

func parseProxyList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	for _, part := range parts {
		proxy := strings.TrimSpace(part)
		if proxy == "" {
			continue
		}
		proxies = append(proxies, proxy)
	}

	return proxies
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
