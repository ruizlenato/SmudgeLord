package main

import (
	"fmt"
	"log"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules"
	"github.com/ruizlenato/smudgelord/internal/telegram"
)

func main() {
	client, err := telegram.Init()
	if err != nil {
		log.Fatal(err)
		return
	}

	if err := initializeServices(); err != nil {
		log.Fatal(err)
	}

	defer func() {
		fmt.Println("[!] — Received stop signal")
		database.Close()
	}()

	err = client.LoginBot(config.TelegramBotToken)
	if err != nil {
		log.Fatalf("\033[31mFailed to login bot:\033[0m %v\n", err)
		return
	}

	botInfo, err := client.GetMe()
	if err != nil {
		log.Fatalf("\033[31mFailed to get bot info:\033[0m %v\n", err)
		return
	}
	fmt.Println("\033[0;32m\U0001F680 Bot Started\033[0m")
	fmt.Printf("\033[0;36mBot Info:\033[0m %v - @%v\n", botInfo.FirstName, botInfo.Username)

	modules.Load(client)

	client.Idle()
}

func initializeServices() error {
	if err := localization.LoadLanguages(); err != nil {
		return fmt.Errorf("load languages: %v", err)
	}

	if err := database.Open(config.DatabaseFile); err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := database.CreateTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	return nil
}
