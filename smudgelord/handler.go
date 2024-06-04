package smudgelord

import (
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/modules/afk"
	"smudgelord/smudgelord/modules/lastfm"
	"smudgelord/smudgelord/modules/medias"
	"smudgelord/smudgelord/modules/menu"
	"smudgelord/smudgelord/modules/misc"
	"smudgelord/smudgelord/modules/stickers"
	"smudgelord/smudgelord/modules/sudoers"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type Handler struct {
	bot *telego.Bot
	bh  *th.BotHandler
}

func NewHandler(bot *telego.Bot, bh *th.BotHandler) *Handler {
	return &Handler{
		bot: bot,
		bh:  bh,
	}
}

func (h *Handler) RegisterHandlers() {
	// Add middleware
	h.bh.Use(database.SaveUsers)
	h.bh.Use(modules.CheckAFK)

	// Add module handlers
	afk.Load(h.bh, h.bot)
	lastfm.Load(h.bh, h.bot)
	medias.Load(h.bh, h.bot)
	menu.Load(h.bh, h.bot)
	misc.Load(h.bh, h.bot)
	stickers.Load(h.bh, h.bot)
	sudoers.Load(h.bh, h.bot)
}
