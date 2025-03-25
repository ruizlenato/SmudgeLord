package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/database"
)

var DisableableCommands []string

func CheckDisabledCommand(command string, chatID int64) bool {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? AND chat_id = ? LIMIT 1);"
	err := database.DB.QueryRow(query, command, chatID).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking command: %v\n", err)
		return false
	}
	return exists
}

func CheckDisabledMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || len(strings.Fields(update.Message.Text)) < 1 || update.Message.Chat.Type == models.ChatTypePrivate {
			next(ctx, b, update)
			return
		}

		if len(strings.Fields(update.Message.Text)) < 1 {
			return
		}

		command := strings.Replace(strings.Fields(update.Message.Text)[0], "/", "", 1)
		if CheckDisabledCommand(command, update.Message.Chat.ID) {
			return
		}

		next(ctx, b, update)
	}
}
