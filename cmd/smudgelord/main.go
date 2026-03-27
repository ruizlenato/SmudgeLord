package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
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
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logger := slog.New(NewColorHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     config.LogLevel,
	}, nil, 0))
	slog.SetDefault(logger)

	botOpts := &gotgbot.BotOpts{}
	if config.BotAPIURL != "" {
		botOpts.BotClient = &gotgbot.BaseBotClient{
			DefaultRequestOpts: &gotgbot.RequestOpts{APIURL: config.BotAPIURL},
		}
	}

	b, err := gotgbot.NewBot(config.TelegramToken, botOpts)
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(NewColorHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     config.LogLevel,
	}, b, config.LogChannelID)))

	if err := initializeServices(b, ctx); err != nil {
		slog.Error("failed to initialize services", "error", err)
		os.Exit(1)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			slog.Error("dispatcher error", "error", err)
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, database.SaveUsers), -1)
	dispatcher.AddHandlerToGroup(handlers.NewCallback(callbackquery.All, database.SaveUsers), -1)
	dispatcher.AddHandlerToGroup(handlers.NewInlineQuery(inlinequery.All, database.SaveUsers), -1)

	modules.RegisterHandlers(dispatcher)

	updater := ext.NewUpdater(dispatcher, nil)

	if config.WebhookURL != "" {
		if err := updater.StartWebhook(b, config.TelegramToken, ext.WebhookOpts{
			ListenAddr: ":" + strconv.Itoa(config.WebhookPort),
		}); err != nil {
			slog.Error("failed to start webhook server", "error", err)
			os.Exit(1)
		}

		if err := updater.SetAllBotWebhooks(config.WebhookURL, &gotgbot.SetWebhookOpts{DropPendingUpdates: true}); err != nil {
			slog.Error("failed to set webhook", "error", err)
			os.Exit(1)
		}
	} else {
		if err := updater.StartPolling(b, &ext.PollingOpts{
			DropPendingUpdates: true,
			GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
				Timeout: 9,
				RequestOpts: &gotgbot.RequestOpts{
					Timeout: 15 * time.Second,
				},
			},
		}); err != nil {
			slog.Error("failed to start polling", "error", err)
			os.Exit(1)
		}
	}

	go func() {
		<-ctx.Done()
		slog.Info("stopping gotgbot updater")
		if err := updater.Stop(); err != nil {
			slog.Error("failed to stop updater", "error", err)
		}
		database.Close()
	}()

	fmt.Printf("Bot started: %s (@%s)\n", b.FirstName, b.Username)
	updater.Idle()
}
