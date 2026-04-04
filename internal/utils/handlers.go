package utils

import (
	"fmt"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/ruizlenato/smudgelord/internal/database"
)

var DisableableCommands []string

var disableableCommandsMu sync.Mutex
var disableableCommandsSet = map[string]struct{}{}

func CheckDisabledCommand(command string, chatID int64) bool {
	command = NormalizeCommand(command)
	if command == "" {
		return false
	}

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? AND chat_id = ? LIMIT 1);"
	err := database.DB.QueryRow(query, command, chatID).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking command: %v\n", err)
		return false
	}
	return exists
}

func NormalizeCommand(command string) string {
	command = strings.TrimSpace(command)
	command = strings.TrimPrefix(command, "/")
	if idx := strings.Index(command, "@"); idx >= 0 {
		command = command[:idx]
	}
	return strings.ToLower(command)
}

type DisableableCommand struct {
	handlers.Command
}

func NewDisableableCommand(command string, response handlers.Response) DisableableCommand {
	normalized := NormalizeCommand(command)
	if normalized != "" {
		disableableCommandsMu.Lock()
		if _, exists := disableableCommandsSet[normalized]; !exists {
			disableableCommandsSet[normalized] = struct{}{}
			DisableableCommands = append(DisableableCommands, normalized)
		}
		disableableCommandsMu.Unlock()
	}

	return DisableableCommand{Command: handlers.NewCommand(normalized, response)}
}

func (d DisableableCommand) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if !d.Command.CheckUpdate(b, ctx) {
		return false
	}

	if ctx == nil || ctx.EffectiveMessage == nil || ctx.EffectiveChat == nil {
		return true
	}

	if ctx.EffectiveChat.Type == gotgbot.ChatTypePrivate {
		return true
	}

	return !CheckDisabledCommand(d.Command.Command, ctx.EffectiveChat.Id)
}
