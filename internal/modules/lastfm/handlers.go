package lastfm

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func getErrorMessage(err error, i18n func(string) string) string {
	switch {
	case strings.Contains(err.Error(), "no recent tracks"):
		return i18n("lastfm.noScrobbles")
	case strings.Contains(err.Error(), "lastFM error"):
		return i18n("lastfm.error")
	default:
		return ""
	}
}

var lastFM = lastFMAPI.Init()

func handleLastFMConfig(bot *telego.Bot, update telego.Update) {
	message := update.CallbackQuery.Message.(*telego.Message)
	lastFMCommands, err := getLastFMCommands(message.Chat.ID)
	if err != nil {
		log.Printf("Error getting lastFMCommands: %v", err)
		return
	}
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
		Text:        i18n("lastfm.configHelp"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

func handleSetUser(bot *telego.Bot, message telego.Message) {
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
			Text:      i18n("lastfm.provideUsername"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if lastFM.GetUser(lastFMUsername) == nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.invalidUsername"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if err := setLastFMUsername(message.From.ID, lastFMUsername); err != nil {
		log.Printf("Error setting lastFM username: %v", err)
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      i18n("lastfm.usernameSet"),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func handleMusic(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	lastFMUsername, err := getUserLastFMUsername(message.From.ID)
	if err != nil || lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.noUsername"),
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
		text += fmt.Sprintf(i18n("lastfm.nowPlaying"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	} else {
		text += fmt.Sprintf(i18n("lastfm.wasPlaying"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
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
			ShowAboveText:    true,
		},
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func handleAlbum(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	lastFMUsername, err := getUserLastFMUsername(message.From.ID)
	if err != nil && lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.noUsername"),
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
		text += fmt.Sprintf(i18n("lastfm.nowPlaying"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	} else {
		text += fmt.Sprintf(i18n("lastfm.wasPlaying"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
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
			ShowAboveText:    true,
		},
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func handleArtist(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}
	i18n := localization.Get(message.Chat)

	lastFMUsername, err := getUserLastFMUsername(message.From.ID)
	if err != nil && lastFMUsername == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm.noUsername"),
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

	body := utils.Request(fmt.Sprintf("https://www.last.fm/music/%s/+images", recentTracks.Artist), utils.RequestParams{
		Method: "GET",
	}).String()

	imageFound := regexp.MustCompile(`https://lastfm.freetls.fastly.net/i/u/avatar170s/[^"]*`).FindStringSubmatch(body)

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)
	if len(imageFound) > 0 {
		text = fmt.Sprintf("<a href='%s'>\u200c</a>", strings.ReplaceAll(imageFound[0], "avatar170s", "770x0")+".jpg")
	}

	if recentTracks.Nowplaying {
		text += fmt.Sprintf(i18n("lastfm.nowPlaying"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	} else {
		text += fmt.Sprintf(i18n("lastfm.wasPlaying"), lastFMUsername, message.From.FirstName, recentTracks.Playcount)
	}

	text += fmt.Sprintf("\n\n🎙<b>%s</b>", recentTracks.Artist)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			PreferLargeMedia: true,
			ShowAboveText:    true,
		},
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("lastfm")
	bh.HandleMessage(handleSetUser, telegohandler.CommandEqual("setuser"))
	bh.HandleMessage(handleMusic, telegohandler.Or(
		telegohandler.CommandEqual("lastfm"),
		telegohandler.CommandEqual("lmu"),
	))
	bh.HandleMessage(handleMusic, telegohandler.Or(telegohandler.CommandEqual("lt"), telegohandler.CommandEqual("np")), lastFMDisabled)
	bh.HandleMessage(handleAlbum, telegohandler.Or(
		telegohandler.CommandEqual("album"),
		telegohandler.CommandEqual("lalb"),
	))
	bh.HandleMessage(handleAlbum, telegohandler.CommandEqual("alb"), lastFMDisabled)
	bh.HandleMessage(handleArtist, telegohandler.Or(
		telegohandler.CommandEqual("artist"),
		telegohandler.CommandEqual("lart")),
	)
	bh.HandleMessage(handleArtist, telegohandler.CommandEqual("art"), lastFMDisabled)
	bh.Handle(handleLastFMConfig, telegohandler.CallbackDataPrefix("lastFMConfig"), helpers.IsAdmin(bot))
}
