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
	bot, err := createBot()
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	var updates <-chan telego.Update

	if config.WebhookURL != "" {
		updates, err = setupWebhook(bot)
	} else {
		updates, err = setupLongPolling(bot)
	}
	if err != nil {
		log.Fatal("Setup updates:", err)
	}

	bh, err := telegohandler.NewBotHandler(bot, updates)
	if err != nil {
		log.Fatal(err)
	}
	handler := smudgelord.NewHandler(bot, bh)
	handler.RegisterHandlers()

	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	if err := initializeServices(); err != nil {
		log.Fatal(err)
	}

	go handleSignals(sigs, bot, bh, done)

	go bh.Start()
	fmt.Println("\033[0;32m\U0001F680 Bot Started\033[0m")
	fmt.Printf("\033[0;36mBot Info:\033[0m %v - @%v\n", botUser.FirstName, botUser.Username)

	if config.WebhookURL != "" {
		go startWebhookServer(bot)
	}

	<-done
	fmt.Println("Done")
}

func createBot() (*telego.Bot, error) {
	var err error
	bot, err := telego.NewBot(config.TelegramToken)
	if config.BotAPIURL != "" {
		bot, err = telego.NewBot(config.TelegramToken, telego.WithAPIServer(config.BotAPIURL))
	}
	return bot, err
}

func setupWebhook(bot *telego.Bot) (<-chan telego.Update, error) {
	err := bot.SetWebhook(&telego.SetWebhookParams{
		DropPendingUpdates: true,
		URL:                config.WebhookURL + bot.Token(),
	})
	if err != nil {
		return nil, fmt.Errorf("set webhook: %w", err)
	}
	return bot.UpdatesViaWebhook("/bot"+bot.Token(),
		telego.WithWebhookServer(telego.FastHTTPWebhookServer{
			Logger: bot.Logger(),
			Server: &fasthttp.Server{},
			Router: router.New(),
		}),
	)
}

func setupLongPolling(bot *telego.Bot) (<-chan telego.Update, error) {
	err := bot.DeleteWebhook(&telego.DeleteWebhookParams{
		DropPendingUpdates: true,
	})
	if err != nil {
		return nil, fmt.Errorf("delete webhook: %w", err)
	}
	return bot.UpdatesViaLongPolling(&telego.GetUpdatesParams{
		Timeout: 4,
	}, telego.WithLongPollingUpdateInterval(0))
}

func initializeServices() error {
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

func handleSignals(sigs chan os.Signal, bot *telego.Bot, bh *telegohandler.BotHandler, done chan struct{}) {
	<-sigs
	fmt.Println("\033[0;31mStopping...\033[0m")

	if config.WebhookURL != "" {
		if err := bot.StopWebhook(); err != nil {
			log.Fatal(err)
		}
	} else {
		bot.StopLongPolling()
	}
	fmt.Println("Updates stopped")

	bh.Stop()
	fmt.Println("Bot handler stopped")

	database.Close()

	done <- struct{}{}
}

func startWebhookServer(bot *telego.Bot) {
	if err := bot.StartWebhook(fmt.Sprintf("0.0.0.0:%d", config.WebhookPort)); err != nil {
		log.Fatal(err)
	}
}
