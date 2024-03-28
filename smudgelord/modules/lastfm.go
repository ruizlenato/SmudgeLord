package modules

import (
	"fmt"
	"log"
	"strings"

	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"
	"smudgelord/smudgelord/utils/lastfm"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

var lastFM = lastfm.Init()

func setUser(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}

	i18n := localization.Get(message.Chat)
	var lastFMUsername string

	if len(strings.Fields(message.Text)) > 1 {
		lastFMUsername = strings.Fields(message.Text)[1]
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("lastfm.provide-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if lastFM.GetUser(lastFMUsername) != nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("lastfm.invalid-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	_, err := database.DB.Exec("UPDATE users SET lastfm_username = ? WHERE id = ?;", lastFMUsername, message.From.ID)
	if err != nil {
		log.Println("Error setting user last.fm username:", err)
		return
	}
	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.From.ID),
		Text:      i18n("lastfm.username-set"),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func music(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	var lastFMUsername string
	row := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", message.From.ID)
	if row.Scan(&lastFMUsername); lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("lastfm.no-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	recentTracks := lastFM.GetRecentTrack(lastFMUsername)
	text := fmt.Sprintf("<a href='%s'>\u200c</a>", (*recentTracks.RecentTracks.Track[0].Image)[3].Text)

	if recentTracks.RecentTracks.Track[0].Attr.Nowplaying != "" {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "track"))
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "track"))
	}

	text += fmt.Sprintf("<b>%s</b> - %s", recentTracks.RecentTracks.Track[0].Artist.Name, recentTracks.RecentTracks.Track[0].Name)
	if recentTracks.RecentTracks.Track[0].Loved == "1" {
		text += " ❤️"
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.From.ID),
		Text:      text,
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func album(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	var lastFMUsername string
	row := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", message.From.ID)
	if row.Scan(&lastFMUsername); lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("lastfm.no-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	recentTracks := lastFM.GetRecentTrack(lastFMUsername)
	text := fmt.Sprintf("<a href='%s'>\u200c</a>", (*recentTracks.RecentTracks.Track[0].Image)[3].Text)

	if recentTracks.RecentTracks.Track[0].Attr.Nowplaying != "" {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "album"))
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "album"))
	}

	text += fmt.Sprintf("<b>%s</b> - %s", recentTracks.RecentTracks.Track[0].Artist.Name, recentTracks.RecentTracks.Track[0].Artist.Name)
	if recentTracks.RecentTracks.Track[0].Loved == "1" {
		text += " ❤️"
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.From.ID),
		Text:      text,
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func LoadLastFM(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("lastfm")
	bh.HandleMessage(setUser, telegohandler.CommandEqual("setuser"))
	bh.HandleMessage(music, telegohandler.CommandEqual("lastfm"))
	bh.HandleMessage(music, telegohandler.CommandEqual("lp"))
	bh.HandleMessage(album, telegohandler.CommandEqual("album"))
	bh.HandleMessage(music, telegohandler.CommandEqual("alb"))
}
