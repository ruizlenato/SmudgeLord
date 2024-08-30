package helpers

import (
	"fmt"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/ruizlenato/smudgelord/internal/database"
)

var DisableableCommands []string

func CheckDisabled(bot *telego.Bot, update telego.Update, next telegohandler.Handler) {
	if update.Message == nil || update.Message.Text == "" || update.Message.Chat.Type == telego.ChatTypePrivate {
		next(bot, update)
		return
	}

	command, _, _ := telegoutil.ParseCommand(update.Message.Text)

	if CheckDisabledCommand(command, update.Message.Chat.ID) {
		return
	}

	next(bot, update)
}

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
