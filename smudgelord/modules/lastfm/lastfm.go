package lastfm

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	lastFMAPI "smudgelord/smudgelord/modules/lastfm/api"
	"smudgelord/smudgelord/utils"
	"smudgelord/smudgelord/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

var lastFM = lastFMAPI.Init()

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

func getUserLastFMUsername(userID int64) (string, error) {
	var lastFMUsername string
	err := database.DB.QueryRow("SELECT lastfm_username FROM users WHERE id = ?;", userID).Scan(&lastFMUsername)
	return lastFMUsername, err
}

func getErrorMessage(err error, i18n func(string) string) string {
	switch {
	case strings.Contains(err.Error(), "no recent tracks"):
		return i18n("lastfm.no-scrobbles")
	case strings.Contains(err.Error(), "lastFM error"):
		return i18n("lastfm.error")
	default:
		return ""
	}
}

func music(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	lastFMUsername, err := getUserLastFMUsername(message.From.ID)
	if err != nil || lastFMUsername == "" {
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

	recentTracks, err := lastFM.GetRecentTrack("track", lastFMUsername)
	if err != nil {
		errorMessage := getErrorMessage(err, i18n)
		if errorMessage != "" {
			bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				Text:      errorMessage,
				ParseMode: "HTML",
				LinkPreviewOptions: &telego.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyParameters: &telego.ReplyParameters{
					MessageID: message.MessageID,
				},
			})
		}
		return
	}

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", (recentTracks.Image))

	if recentTracks.Nowplaying {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	}

	text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Track)
	if recentTracks.Trackloved {
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

	recentTracks, err := lastFM.GetRecentTrack("album", lastFMUsername)
	if err != nil {
		errorMessage := getErrorMessage(err, i18n)
		if errorMessage != "" {
			bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				Text:      errorMessage,
				ParseMode: "HTML",
				LinkPreviewOptions: &telego.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyParameters: &telego.ReplyParameters{
					MessageID: message.MessageID,
				},
			})
		}
		return
	}

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)

	if recentTracks.Nowplaying {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	}

	text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Album)
	if recentTracks.Trackloved {
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

	recentTracks, err := lastFM.GetRecentTrack("artist", lastFMUsername)
	if err != nil {
		errorMessage := getErrorMessage(err, i18n)
		if errorMessage != "" {
			bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				Text:      errorMessage,
				ParseMode: "HTML",
				LinkPreviewOptions: &telego.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyParameters: &telego.ReplyParameters{
					MessageID: message.MessageID,
				},
			})
		}
		return
	}

	body := utils.RequestGET(fmt.Sprintf("https://www.last.fm/music/%s/+images", recentTracks.Artist), utils.RequestGETParams{}).String()
	imageFound := regexp.MustCompile(`https://lastfm.freetls.fastly.net/i/u/avatar170s/[^"]*`).FindStringSubmatch(body)

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)
	if len(imageFound) > 0 {
		text = fmt.Sprintf("<a href='%s'>\u200c</a>", strings.ReplaceAll(imageFound[0], "avatar170s", "770x0")+".jpg")
	}

	if recentTracks.Nowplaying {
		text += fmt.Sprintf(i18n("lastfm.now-playing"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	} else {
		text += fmt.Sprintf(i18n("lastfm.was-playing"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	}

	text += fmt.Sprintf("\n\n🎙<b>%s</b>", recentTracks.Artist)

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

func lastFMDisabled(update telego.Update) bool {
	var lastFMCommands bool = true
	message := update.Message
	if message.Chat.Type == telego.ChatTypePrivate {
		return lastFMCommands
	}

	database.DB.QueryRow("SELECT lastFMCommands FROM groups WHERE id = ?;", message.Chat.ID).Scan(&lastFMCommands)
	return lastFMCommands
}

func LoadLastFM(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("lastfm")
	bh.HandleMessage(setUser, telegohandler.CommandEqual("setuser"))
	bh.HandleMessage(music, telegohandler.Or(
		telegohandler.CommandEqual("lastfm"),
		telegohandler.CommandEqual("lmu"),
	))
	bh.HandleMessage(music, telegohandler.Or(telegohandler.CommandEqual("lt"), telegohandler.CommandEqual("np")), lastFMDisabled)
	bh.HandleMessage(album, telegohandler.Or(
		telegohandler.CommandEqual("album"),
		telegohandler.CommandEqual("lalb"),
	))
	bh.HandleMessage(album, telegohandler.CommandEqual("alb"), lastFMDisabled)
	bh.HandleMessage(artist, telegohandler.Or(
		telegohandler.CommandEqual("artist"),
		telegohandler.CommandEqual("lart")),
	)
	bh.HandleMessage(artist, telegohandler.CommandEqual("art"), lastFMDisabled)
	bh.Handle(lastFMConfig, telegohandler.CallbackDataPrefix("lastFMConfig"), helpers.IsAdmin(bot))
}
