package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"smudgelord/smudgelord"
	"smudgelord/smudgelord/config"
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"

	"github.com/fasthttp/router"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/valyala/fasthttp"
)

func main() {
	var bot *telego.Bot
	var err error

	// Create bot
	if config.BotAPIURL != "" {
		bot, err = telego.NewBot(config.TelegramToken, telego.WithAPIServer(config.BotAPIURL))
	} else {
		bot, err = telego.NewBot(config.TelegramToken)
	}
	if err != nil {
		log.Fatal(err)
	}

	// Initialize signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{}, 1)

	var updates <-chan telego.Update

	// Check if the webhook URL is empty.
	// If the webhook URL is empty, the bot will get the updates via long polling
	if config.WebhookURL != "" {
		err = bot.SetWebhook(&telego.SetWebhookParams{
			DropPendingUpdates: true,
			URL:                config.WebhookURL + bot.Token(),
		})
		if err != nil {
			log.Fatal("Set webhook:", err)
		}

		// Get updates using the webhook.
		updates, err = bot.UpdatesViaWebhook("/bot"+bot.Token(),
			telego.WithWebhookServer(telego.FastHTTPWebhookServer{
				Logger: bot.Logger(),
				Server: &fasthttp.Server{},
				Router: router.New(),
			}),
		)
	} else {
		// Delete the webhook for the Telegram bot, specifying that any pending updates should be dropped.
		err = bot.DeleteWebhook(&telego.DeleteWebhookParams{
			DropPendingUpdates: true,
		})
		if err != nil {
			log.Fatal("Delete webhook:", err)
		}
		// Get updates using long polling.
		updates, err = bot.UpdatesViaLongPolling(&telego.GetUpdatesParams{
			Timeout: 4,
		}, telego.WithLongPollingUpdateInterval(0))
	}

	if err != nil {
		log.Fatal("Get updates:", err)
	}

	// Handle updates
	bh, err := telegohandler.NewBotHandler(bot, updates)
	if err != nil {
		log.Fatal(err)
	}
	handler := smudgelord.NewHandler(bot, bh)
	handler.RegisterHandlers()

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
	if err := database.Open(config.DatabaseFile); err != nil {
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
		if config.WebhookURL == "" {
			bot.StopLongPolling()
		} else {
			err = bot.StopWebhook()
		}
		if err != nil {
			log.Fatal(err)
		}
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

	// Start server for receiving requests from the Telegram
	if config.WebhookURL != "" {
		go func() {
			err = bot.StartWebhook(fmt.Sprintf("0.0.0.0:%d", config.WebhookPort))
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	<-done
	fmt.Println("Done")
}
