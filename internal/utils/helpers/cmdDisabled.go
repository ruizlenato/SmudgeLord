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
	if update.Message == nil || update.Message.Text == "" {
		next(bot, update)
		return
	}

	command, _, _ := telegoutil.ParseCommand(update.Message.Text)
	checkDisabledCommand := func(command string) bool {
		var exists bool
		query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? LIMIT 1);"
		err := database.DB.QueryRow(query, command).Scan(&exists)
		if err != nil {
			fmt.Printf("Error checking command: %v\n", err)
			return false
		}
		return exists
	}

	if checkDisabledCommand(command) {
		return
	}

	next(bot, update)
	return
}
