package modules

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils"
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
			ChatID:    telegoutil.ID(message.Chat.ID),
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
		log.Print("[lastfm/setUser] Error setting user last.fm username:", err)
		return
	}
	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
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
	err := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", message.From.ID).Scan(&lastFMUsername)
	if err != nil && lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.no-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	recentTracks := lastFM.GetRecentTrack(lastFMUsername)
	if recentTracks == nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.error"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}
	if recentTracks.RecentTracks == nil || len(*recentTracks.RecentTracks.Track) < 1 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.no-scrobbles"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}
	text := fmt.Sprintf("<a href='%s'>\u200c</a>", (*recentTracks.RecentTracks.Track)[0].Image[3].Text)

	if (*recentTracks.RecentTracks.Track)[0].Attr.Nowplaying != "" {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "track"))
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "track"))
	}

	text += fmt.Sprintf("<b>%s</b> - %s", (*recentTracks.RecentTracks.Track)[0].Artist.Name, (*recentTracks.RecentTracks.Track)[0].Name)
	if (*recentTracks.RecentTracks.Track)[0].Loved == "1" {
		text += " ❤️"
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			PreferLargeMedia: true,
		},
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
	err := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", message.From.ID).Scan(&lastFMUsername)
	if err != nil && lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.no-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	recentTracks := lastFM.GetRecentTrack(lastFMUsername)
	if recentTracks == nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.error"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}
	if recentTracks.RecentTracks == nil || len(*recentTracks.RecentTracks.Track) < 1 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.no-scrobbles"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}
	text := fmt.Sprintf("<a href='%s'>\u200c</a>", (*recentTracks.RecentTracks.Track)[0].Image[3].Text)

	if (*recentTracks.RecentTracks.Track)[0].Attr.Nowplaying != "" {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "album"))
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "album"))
	}

	text += fmt.Sprintf("<b>%s</b> - %s", (*recentTracks.RecentTracks.Track)[0].Artist.Name, (*recentTracks.RecentTracks.Track)[0].Album.Text)
	if (*recentTracks.RecentTracks.Track)[0].Loved == "1" {
		text += " ❤️"
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			PreferLargeMedia: true,
		},
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func artist(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	var lastFMUsername string
	err := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", message.From.ID).Scan(&lastFMUsername)
	if err != nil && lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.no-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	recentTracks := lastFM.GetRecentTrack(lastFMUsername)
	if recentTracks == nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.error"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}
	if recentTracks.RecentTracks == nil || len(*recentTracks.RecentTracks.Track) < 1 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.no-scrobbles"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}
	body := utils.RequestGET(fmt.Sprintf("https://www.last.fm/music/%s/+images", (*recentTracks.RecentTracks.Track)[0].Artist.Name), utils.RequestGETParams{}).String()
	imageFound := regexp.MustCompile(`https://lastfm.freetls.fastly.net/i/u/avatar170s/[^"]*`).FindStringSubmatch(body)

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", (*recentTracks.RecentTracks.Track)[0].Image[3].Text)
	if len(imageFound) > 0 {
		text = fmt.Sprintf("<a href='%s'>\u200c</a>", strings.ReplaceAll(imageFound[0], "avatar170s", "770x0")+".jpg")
	}

	if (*recentTracks.RecentTracks.Track)[0].Attr.Nowplaying != "" {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "artist"))
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, lastFM.PlayCount(recentTracks, "artist"))
	}

	text += fmt.Sprintf("🎙<b>%s</b>", (*recentTracks.RecentTracks.Track)[0].Artist.Name)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			PreferLargeMedia: true,
		},
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func lastFMConfig(bot *telego.Bot, update telego.Update) {
	var lastFMCommands bool
	message := update.CallbackQuery.Message.(*telego.Message)
	database.DB.QueryRow("SELECT lastFMCommands FROM groups WHERE id = ?;", message.Chat.ID).Scan(&lastFMCommands)
	chat := message.GetChat()
	i18n := localization.Get(chat)

	configType := strings.ReplaceAll(update.CallbackQuery.Data, "lastFMConfig ", "")
	if configType != "lastFMConfig" {
		lastFMCommands = !lastFMCommands
		_, err := database.DB.Exec("UPDATE groups SET lastFMCommands = ? WHERE id = ?;", lastFMCommands, message.Chat.ID)
		if err != nil {
			return
		}
	}

	state := func(state bool) string {
		if state {
			return "✅"
		}
		return "☑️"
	}

	buttons := [][]telego.InlineKeyboardButton{
		{
			{Text: state(lastFMCommands), CallbackData: "lastFMConfig update"},
		},
	}

	buttons = append(buttons, []telego.InlineKeyboardButton{{
		Text:         i18n("button.back"),
		CallbackData: "configMenu",
	}})

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(chat.ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("lastfm.config-help"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

func LoadLastFM(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("lastfm")
	bh.HandleMessage(setUser, telegohandler.CommandEqual("setuser"))
	bh.HandleMessage(music, telegohandler.Or(
		telegohandler.CommandEqual("lastfm"),
		telegohandler.CommandEqual("lt"),
		telegohandler.CommandEqual("np"),
		telegohandler.CommandEqual("lmu"),
	))
	bh.HandleMessage(album, telegohandler.Or(
		telegohandler.CommandEqual("album"),
		telegohandler.CommandEqual("alb"),
		telegohandler.CommandEqual("lalb"),
	))
	bh.HandleMessage(artist, telegohandler.Or(
		telegohandler.CommandEqual("artist"),
		telegohandler.CommandEqual("art"),
		telegohandler.CommandEqual("lart")),
	)
	bh.Handle(lastFMConfig, telegohandler.CallbackDataPrefix("lastFMConfig"), helpers.IsAdmin(bot))
}
