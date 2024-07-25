package telegram

import (
	"context"
	"fmt"

	"smudgelord/internal/database"
	"smudgelord/internal/modules/afk"
	"smudgelord/internal/modules/lastfm"
	"smudgelord/internal/modules/medias"
	"smudgelord/internal/modules/menu"
	"smudgelord/internal/modules/misc"
	"smudgelord/internal/modules/stickers"
	"smudgelord/internal/modules/sudoers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
)

func BotHandler(ctx context.Context, bot *telego.Bot, updates <-chan telego.Update) (*telegohandler.BotHandler, error) {
	bh, err := telegohandler.NewBotHandler(bot, updates)
	if err != nil {
		return nil, fmt.Errorf("create bot handler: %w", err)
	}

	return bh, nil
}

type Handler struct {
	bot *telego.Bot
	bh  *telegohandler.BotHandler
}

func NewHandler(bot *telego.Bot, bh *telegohandler.BotHandler) *Handler {
	return &Handler{
		bot: bot,
		bh:  bh,
	}
}

func (h *Handler) RegisterHandlers() {
	h.bh.Use(database.SaveUsers)

	afk.Load(h.bh, h.bot)
	lastfm.Load(h.bh, h.bot)
	medias.Load(h.bh, h.bot)
	menu.Load(h.bh, h.bot)
	misc.Load(h.bh, h.bot)
	stickers.Load(h.bh, h.bot)
	sudoers.Load(h.bh, h.bot)
}
