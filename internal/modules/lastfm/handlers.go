package lastfm

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func getErrorMessage(err error, i18n func(string, ...map[string]interface{}) string) string {
	switch {
	case strings.Contains(err.Error(), "no recent tracks"):
		return i18n("no-scrobbled-yet")
	case strings.Contains(err.Error(), "lastFM error"):
		return i18n("lastfm-error")
	default:
		return ""
	}
}

var lastFM = lastFMAPI.Init()

func handleSetUser(bot *telego.Bot, message telego.Message) {
	if strings.Contains(message.Chat.Type, "group") && message.From.ID == message.Chat.ID {
		return
	}

	i18n := localization.Get(message)
	var lastFMUsername string

	if len(strings.Fields(message.Text)) > 1 {
		lastFMUsername = strings.Fields(message.Text)[1]
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("no-lastfm-username-provided"),
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
			Text:      i18n("invalid-lastfm-username"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if err := setLastFMUsername(message.From.ID, lastFMUsername); err != nil {
		slog.Error("Couldn't set lastFM username: %v", "Error", err.Error())
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      i18n("lastfm-username-saved"),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func handleMusic(bot *telego.Bot, message telego.Message) {
	lastfm(bot, message, "track")
	return
}

func handleAlbum(bot *telego.Bot, message telego.Message) {
	lastfm(bot, message, "album")
	return
}

func handleArtist(bot *telego.Bot, message telego.Message) {
	lastfm(bot, message, "artist")
	return
}

func lastfm(bot *telego.Bot, message telego.Message, methodType string) {
	i18n := localization.Get(message)
	lastFMUsername, err := getUserLastFMUsername(message.From.ID)
	if err != nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("lastfm-username-not-defined"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	recentTracks, err := lastFM.GetRecentTrack(methodType, lastFMUsername)
	if err != nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      getErrorMessage(err, i18n),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)
	text += i18n("lastfm-playing", map[string]interface{}{
		"nowplaying":     fmt.Sprintf("%v", recentTracks.Nowplaying),
		"lastFMUsername": lastFMUsername,
		"firstName":      message.From.FirstName,
		"playcount":      recentTracks.Playcount,
	})

	switch methodType {
	case "track":
		text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Track)
		if recentTracks.Trackloved {
			text += " ❤️"
		}
	case "album":
		text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Album)
	case "artist":
		text += fmt.Sprintf("\n\n🎙<b>%s</b>", recentTracks.Artist)
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

	return
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("lastfm")
	bh.HandleMessage(handleSetUser, telegohandler.CommandEqual("setuser"))
	bh.HandleMessage(handleMusic, telegohandler.Or(
		telegohandler.CommandEqual("lastfm"),
		telegohandler.CommandEqual("lt"),
		telegohandler.CommandEqual("np"),
		telegohandler.CommandEqual("lmu"),
	))
	bh.HandleMessage(handleAlbum, telegohandler.Or(
		telegohandler.CommandEqual("album"),
		telegohandler.CommandEqual("alb"),
		telegohandler.CommandEqual("lalb"),
	))
	bh.HandleMessage(handleArtist, telegohandler.Or(
		telegohandler.CommandEqual("artist"),
		telegohandler.CommandEqual("art"),
		telegohandler.CommandEqual("lart")),
	)
	helpers.DisableableCommands = append(helpers.DisableableCommands, "lastfm", "lmu", "lt", "np", "album", "lalb", "alb", "artist", "lart", "art")
}
