package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"smudgelord/smudgelord"
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"syscall"

	"github.com/caarlos0/env/v10"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
)

type config struct {
	TelegramToken string `env:"TELEGRAM_TOKEN" validate:"required"`
	DatabaseFile  string `env:"DATABASE_FILE" validate:"required"`
}

func main() {
	// Get Bot from environment variables (.env)
	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("%+v\n", err)
	}

	// Create bot
	bot, err := telego.NewBot(cfg.TelegramToken)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{}, 1)

	// Get updates
	updates, _ := bot.UpdatesViaLongPolling(nil)

	// Handle updates
	bh, _ := telegohandler.NewBotHandler(bot, updates)
	handler := smudgelord.NewHandler(bot, bh)
	handler.RegisterHandlers()
	bot.DeleteWebhook(&telego.DeleteWebhookParams{
		DropPendingUpdates: true,
	})

	// Call method getMe
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	if err := localization.LoadLanguages(); err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Open a new SQLite database file
	if err := database.Open(cfg.DatabaseFile); err != nil {
		log.Fatal(err)
	}

	// Define the tables
	if err := database.CreateTables(); err != nil {
		log.Fatal("Error creating table:", err)
		return
	}

	go func() {
		// Wait for stop signal
		<-sigs
		fmt.Println("\033[0;31mStopping...\033[0m")

		bot.StopLongPolling()
		fmt.Println("Long polling stopped")

		bh.Stop()
		fmt.Println("Bot handler stopped")

		// Close the database connection
		database.Close()

		done <- struct{}{}
	}()

	go bh.Start()
	fmt.Println("\033[0;32m\U0001F680 Bot Started\033[0m")
	fmt.Printf("\033[0;36mBot Info:\033[0m %v - @%v\n", botUser.FirstName, botUser.Username)

	<-done
	fmt.Println("Done")
}
