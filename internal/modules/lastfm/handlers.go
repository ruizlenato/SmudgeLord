package lastfm

import (
	"fmt"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
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

func handleSetUser(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	if message.Args() != "" {
		if err := lastFM.GetUser(message.Args()); err != nil {
			_, err := message.Reply(i18n("invalid-lastfm-username"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}

		if err := setLastFMUsername(message.SenderID(), message.Args()); err != nil {
			_, err := message.Reply(i18n("lastfm-error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}

		_, err := message.Reply(i18n("lastfm-username-saved"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	conv, err := message.Conv()
	if err != nil {
		return err
	}
	defer conv.Close()

	respond, err := conv.Respond(i18n("no-lastfm-username-provided"), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: &telegram.ReplyKeyboardForceReply{},
	})
	if err != nil {
		return err
	}

	resp, err := conv.GetResponse()
	if err != nil {
		return err
	}

	if respond.ID == resp.ReplyToMsgID() {
		if err := lastFM.GetUser(resp.Text()); err != nil {
			_, err := conv.Reply(i18n("invalid-lastfm-username"), &telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}

		if err := setLastFMUsername(message.SenderID(), resp.Text()); err != nil {
			_, err := message.Reply(i18n("lastfm-error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}

		_, err = conv.Respond(i18n("lastfm-username-saved"), &telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	_, err = respond.Delete()
	if respond.IsReply() {
		reply, err := respond.GetReplyMessage()
		if err != nil {
			return err
		}
		_, err = reply.Delete()
		return err
	}

	return err
}

func handleMusic(message *telegram.NewMessage) error {
	return lastfm(message, "track")
}

func handleAlbum(message *telegram.NewMessage) error {
	return lastfm(message, "album")
}

func handleArtist(message *telegram.NewMessage) error {
	return lastfm(message, "artist")
}

func lastfm(message *telegram.NewMessage, methodType string) error {
	i18n := localization.Get(message)
	lastFMUsername, err := getUserLastFMUsername(message.SenderID())
	if err != nil {
		_, err := message.Reply(i18n("lastfm-error"), telegram.SendOptions{ParseMode: telegram.HTML})
		return err
	}

	recentTracks, err := lastFM.GetRecentTrack(methodType, lastFMUsername)
	if err != nil {
		_, err := message.Reply(getErrorMessage(err, i18n), telegram.SendOptions{ParseMode: telegram.HTML})
		return err
	}

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)
	text += i18n("lastfm-playing", map[string]interface{}{
		"nowplaying":     fmt.Sprintf("%v", recentTracks.Nowplaying),
		"lastFMUsername": lastFMUsername,
		"firstName":      message.Sender.FirstName,
		"playcount":      recentTracks.Playcount,
	})

	switch methodType {
	case "track":
		text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Track)
		if recentTracks.Trackloved {
			text += i18n("lastfm.trackLoved")
		}
	case "album":
		text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Album)
	case "artist":
		text += fmt.Sprintf("\n\n🎙<b>%s</b>", recentTracks.Artist)
	}

	_, err = message.Reply(text, telegram.SendOptions{
		ParseMode:   telegram.HTML,
		InvertMedia: true,
		LinkPreview: true,
	})

	return err
}

func Load(client *telegram.Client) {
	utils.SotreHelp("lastfm")
	client.On("command:setuser", handlers.HandleCommand(handleSetUser))
	client.On("command:lastfm", handlers.HandleCommand(handleMusic))
	client.On("command:lt", handlers.HandleCommand(handleMusic))
	client.On("command:lmu", handlers.HandleCommand(handleMusic))
	client.On("command:album", handlers.HandleCommand(handleAlbum))
	client.On("command:lalb", handlers.HandleCommand(handleAlbum))
	client.On("command:alb", handlers.HandleCommand(handleAlbum))
	client.On("command:artist", handlers.HandleCommand(handleArtist))
	client.On("command:lart", handlers.HandleCommand(handleArtist))
	client.On("command:art", handlers.HandleCommand(handleArtist))

	handlers.DisableableCommands = append(handlers.DisableableCommands,
		"setuser", "lastfm", "lt", "lmu", "album", "lalb", "alb", "artist", "lart", "art")
}
