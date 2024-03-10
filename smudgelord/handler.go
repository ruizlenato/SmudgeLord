package smudgelord

import (
	"regexp"
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
	h.bh.Use(database.SaveUsers)
	h.bh.Use(modules.CheckAFK)
	h.bh.HandleMessage(modules.SetAFK, th.CommandEqual("afk"))
	h.bh.HandleMessage(modules.SetAFK, th.TextMatches(regexp.MustCompile(`^(?:brb)(\s.+)?`)))
	h.bh.HandleMessage(modules.MediaDownloader, th.TextMatches(regexp.MustCompile(`(?:htt.*?//)?(:?.*)?(?:instagram|twitter|x|tiktok|threads)\.(?:com|net)\/(?:\S*)`)))
	h.bh.Handle(modules.Start, th.CommandEqual("start"))
}
