package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/modules"
	"github.com/ruizlenato/smudgelord/internal/telegram"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
)

func main() {
	bot, err := telegram.CreateBot()
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	var updates <-chan telego.Update

	if config.WebhookURL != "" {
		updates, err = telegram.SetupWebhook(bot)
	} else {
		updates, err = telegram.SetupLongPolling(bot)
	}
	if err != nil {
		log.Fatal("Setup updates:", err)
	}

	bh, err := telegohandler.NewBotHandler(bot, updates)
	if err != nil {
		log.Fatal(err)
	}
	handler := modules.NewHandler(bot, bh)

	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	if err := InitializeServices(); err != nil {
		log.Fatal(err)
	}

	go handleSignals(sigs, bot, bh, done)

	go bh.Start()
	fmt.Println("\033[0;32m\U0001F680 Bot Started\033[0m")
	fmt.Printf("\033[0;36mBot Info:\033[0m %v - @%v\n", botUser.FirstName, botUser.Username)
	handler.RegisterHandlers()

	if config.WebhookURL != "" {
		go StartWebhookServer(bot)
	}

	<-done
	fmt.Println("Done")
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
