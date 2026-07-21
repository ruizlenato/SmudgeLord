package utils

import (
	"log/slog"
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

var disabledCmdsCache = map[int64]map[string]struct{}{}
var disabledCmdsCacheMu sync.RWMutex

func CheckDisabledCommand(command string, chatID int64) bool {
	command = NormalizeCommand(command)
	if command == "" {
		return false
	}

	disabledCmdsCacheMu.RLock()
	set, ok := disabledCmdsCache[chatID]
	disabledCmdsCacheMu.RUnlock()
	if ok {
		_, disabled := set[command]
		return disabled
	}

	set, err := loadDisabledCommands(chatID)
	if err != nil {
		slog.Warn("CheckDisabledCommand: failed to load disabled commands",
			"chatID", chatID, "err", err)
		return false
	}

	disabledCmdsCacheMu.Lock()
	disabledCmdsCache[chatID] = set
	disabledCmdsCacheMu.Unlock()

	_, disabled := set[command]
	return disabled
}

func loadDisabledCommands(chatID int64) (map[string]struct{}, error) {
	rows, err := database.DB.Query("SELECT command FROM commandsDisabled WHERE chat_id = ?;", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := map[string]struct{}{}
	for rows.Next() {
		var cmd string
		if err := rows.Scan(&cmd); err != nil {
			return nil, err
		}
		set[cmd] = struct{}{}
	}
	return set, rows.Err()
}

func InvalidateDisabledCommandsCache(chatID int64) {
	disabledCmdsCacheMu.Lock()
	delete(disabledCmdsCache, chatID)
	disabledCmdsCacheMu.Unlock()
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
