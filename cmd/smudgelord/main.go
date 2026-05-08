package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/inlinequery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/modules"
	"github.com/ruizlenato/smudgelord/internal/utils/conversation"
)

func main() {
	beta := flag.Bool("beta", false, "disable Telegram log channel forwarding")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	conversation.SetShutdownContext(ctx)

	logger := slog.New(NewColorHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     config.LogLevel,
	}, nil, 0))
	slog.SetDefault(logger)

	botOpts := &gotgbot.BotOpts{}
	if config.BotAPIURL != "" {
		botOpts.BotClient = &gotgbot.BaseBotClient{
			DefaultRequestOpts: &gotgbot.RequestOpts{
				Timeout: 500 * time.Second,
				APIURL:  config.BotAPIURL,
			},
		}
	}

	b, err := gotgbot.NewBot(config.TelegramToken, botOpts)
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	logChatID := config.LogChannelID
	if *beta {
		logChatID = 0
	}

	slog.SetDefault(slog.New(NewColorHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     config.LogLevel,
	}, b, logChatID)))

	if err := initializeServices(b, ctx, logChatID); err != nil {
		slog.Error("failed to initialize services", "error", err.Error())
		os.Exit(1)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			slog.Error("dispatcher error", "error", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, database.SaveUsers).SetAllowChannel(true), -1)
	dispatcher.AddHandlerToGroup(handlers.NewCallback(callbackquery.All, database.SaveUsers), -1)
	dispatcher.AddHandlerToGroup(handlers.NewInlineQuery(inlinequery.All, database.SaveUsers), -1)

	modules.RegisterHandlers(dispatcher)

	updater := ext.NewUpdater(dispatcher, nil)

	var closeDBOnce sync.Once
	closeDatabase := func() {
		closeDBOnce.Do(func() {
			database.Close()
		})
	}

	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	go func() {
		<-interrupts
		<-interrupts

		slog.Warn("forced shutdown requested, exiting now")

		dbClosed := make(chan struct{})
		go func() {
			closeDatabase()
			close(dbClosed)
		}()

		select {
		case <-dbClosed:
		case <-time.After(2 * time.Second):
			slog.Warn("database close timed out on forced shutdown")
		}

		os.Exit(1)
	}()

	if config.WebhookURL != "" {
		if err := updater.StartWebhook(b, config.TelegramToken, ext.WebhookOpts{
			ListenAddr: ":" + strconv.Itoa(config.WebhookPort),
		}); err != nil {
			slog.Error("failed to start webhook server", "error", err)
			os.Exit(1)
		}

		if err := updater.SetAllBotWebhooks(config.WebhookURL, &gotgbot.SetWebhookOpts{DropPendingUpdates: true, AllowedUpdates: []string{}}); err != nil {
			slog.Error("failed to set webhook", "error", err)
			os.Exit(1)
		}
	} else {
		if err := updater.StartPolling(b, &ext.PollingOpts{
			DropPendingUpdates: true,
			GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
				Timeout:        9,
				AllowedUpdates: []string{},
				RequestOpts: &gotgbot.RequestOpts{
					Timeout: 15 * time.Second,
				},
			},
		}); err != nil {
			slog.Error("failed to start polling", "error", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Bot started: %s (@%s)\n", b.FirstName, b.Username)

	<-ctx.Done()
	slog.Info("stopping gotgbot updater")

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- updater.Stop()
	}()

	select {
	case err := <-stopDone:
		if err != nil {
			slog.Error("failed to stop updater", "error", err)
		}
	case <-time.After(20 * time.Second):
		slog.Warn("updater stop timed out, waiting for second Ctrl+C to force exit")
	}

	closeDatabase()
}
