package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/config"
)

var (
	Client *telegram.Client
	once   sync.Once
)

func Init() (*telegram.Client, error) {
	var err error

	once.Do(func() {
		Client, err = createClient()
	})

	if err != nil {
		return nil, err
	}

	return Client, nil
}

func createClient() (*telegram.Client, error) {
	if config.TelegramAPIID == 0 || config.TelegramAPIHash == "" {
		return nil, fmt.Errorf("telegram API credentials not configured")
	}

	client, err := telegram.NewClient(telegram.ClientConfig{
		AppID:        config.TelegramAPIID,
		AppHash:      config.TelegramAPIHash,
		LogLevel:     telegram.LogError,
		FloodHandler: handleFlood,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Telegram client: %w", err)
	}

	return client, nil
}

func handleFlood(err error) bool {
	wait := telegram.GetFloodWait(err)
	if wait > 0 {
		slog.Info("Flood wait applied", "seconds", wait)
		time.Sleep(time.Duration(wait) * time.Second)
		return true
	}
	return false
}

func GetClient() *telegram.Client {
	return Client
}

func Close(ctx context.Context) error {
	if Client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Client.Disconnect()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout while closing client: %w", ctx.Err())
	}
}
