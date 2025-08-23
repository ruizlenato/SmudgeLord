package telegram

import (
	"fmt"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/config"
)

var Client *telegram.Client

func Init() (*telegram.Client, error) {
	client, err := telegram.NewClient(telegram.ClientConfig{
		AppID:    config.TelegramAPIID,
		AppHash:  config.TelegramAPIHash,
		LogLevel: telegram.LogError,
	})
	if err != nil {
		return nil, fmt.Errorf("\033[31mFailed to initialize Telegram client:\033[0m %v", err)
	}

	Client = client

	return client, nil
}
