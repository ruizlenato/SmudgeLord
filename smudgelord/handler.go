package smudgelord

import (
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/modules"

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
	modules.LoadStart(h.bh, h.bot)
	modules.LoadAFK(h.bh, h.bot)
	modules.LoadLastFM(h.bh, h.bot)
	modules.LoadMediaDownloader(h.bh, h.bot)
	modules.LoadStickers(h.bh, h.bot)
}
