package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/modules"
	"github.com/ruizlenato/smudgelord/internal/modules/afk"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var botInfo *models.User

func main() {
	logger := slog.New(NewColorHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     config.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	done := make(chan struct{}, 1)

	go handleSignals(ctx, done)

	opts := []bot.Option{
		bot.WithMiddlewares(
			database.SaveUsers,
			afk.CheckAFKMiddleware,
			utils.CheckDisabledMiddleware,
			checkUsername,
		),
	}

	if config.BotAPIURL != "" {
		opts = append(opts, bot.WithServerURL(config.BotAPIURL))
	}

	b, err := bot.New(config.TelegramToken, opts...)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if err := InitializeServices(b, ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	botInfo, err = b.GetMe(ctx)
	if err != nil {
		slog.Error("failed to get bot info",
			"error", err.Error())
		os.Exit(1)
	}

	fmt.Println("\033[0;32m\U0001F680 Bot Started\033[0m")
	fmt.Printf("\033[0;36mBot Info:\033[0m %v - @%v\n", botInfo.FirstName, botInfo.Username)
	modules.RegisterHandlers(b)

	if config.WebhookURL != "" {
		b.StartWebhook(ctx)
	} else {
		b.Start(ctx)
	}

	<-done
	fmt.Println("Done")
}

func handleSignals(ctx context.Context, done chan struct{}) {
	<-ctx.Done()
	fmt.Println("\n\033[0;31mStopping...\033[0m")

	fmt.Println("Bot stopped")
	database.Close()

	done <- struct{}{}
}

func checkUsername(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			next(ctx, b, update)
			return
		}

		commandSlices := strings.Split(update.Message.Text, "@")
		if len(commandSlices) > 1 && strings.HasPrefix(commandSlices[0], "/") {
			if commandSlices[1] != botInfo.Username {
				return
			}
		}

		next(ctx, b, update)
	}
}
