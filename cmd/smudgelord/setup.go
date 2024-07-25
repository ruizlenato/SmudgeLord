package main

import (
	"fmt"
	"log"

	"smudgelord/internal/config"
	"smudgelord/internal/database"
	"smudgelord/internal/localization"

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

	return nil
}

func StartWebhookServer(bot *telego.Bot) {
	go func() {
		if err := bot.StartWebhook(fmt.Sprintf("0.0.0.0:%d", config.WebhookPort)); err != nil {
			log.Fatal(err)
		}
	}()
}
