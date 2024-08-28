package main

import (
	"fmt"
	"log"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"

	"github.com/mymmrac/telego"
)

func InitializeServices() error {
	if err := localization.LoadLanguages(); err != nil {
		return fmt.Errorf("load languages: %w", err)
	}

	if err := database.Open(config.DatabaseFile); err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := database.CreateTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	if err := cache.RedisClient("localhost:6379", "", 0); err != nil {
		log.Println("\033[0;31mRedis cache is currently unavailable.\033[0m")
	}

	return nil
}

func StartWebhookServer(bot *telego.Bot) {
	if err := bot.StartWebhook(fmt.Sprintf("0.0.0.0:%d", config.WebhookPort)); err != nil {
		log.Fatal(err)
	}
}
