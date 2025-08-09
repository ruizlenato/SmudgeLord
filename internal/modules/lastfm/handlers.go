package lastfm

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func getErrorMessage(err error, i18n func(string, ...map[string]any) string) string {
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

func SetUserHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	if message.Args() != "" && message.Args() != "setuser" {
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
			slog.Error(
				"Could not set lastfm username",
				"error", err.Error(),
			)
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

	respond, err := conv.Respond(i18n("reply-with-lastfm-username"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyID:   message.ID,
		ReplyMarkup: &telegram.ReplyKeyboardForceReply{
			SingleUse: true,
			Selective: true,
		},
	})
	if err != nil {
		return err
	}

	resp, err := conv.GetResponse()
	if err != nil {
		return err
	}

	getRespReply, err := resp.GetReplyMessage()
	if getRespReply == nil || err != nil {
		respond.Delete()
		_, err := conv.Respond(i18n("didnt-replied-with-lastfm-username"), &telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyID:   message.ID,
		})
		return err
	}

	for resp.Sender.ID != message.Sender.ID {
		resp, err = conv.GetReply()
		if err != nil {
			return err
		}
	}

	if err := lastFM.GetUser(resp.Text()); err != nil {
		_, err := message.Reply(i18n("invalid-lastfm-username"), telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyID:   resp.ID,
		})
		return err
	}

	if err := setLastFMUsername(message.SenderID(), resp.Text()); err != nil {
		_, err := message.Reply(i18n("lastfm-error"), telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyID:   resp.ID,
		})
		slog.Error(
			"Could not set lastfm username",
			"error", err.Error(),
		)
		return err
	}

	_, err = conv.Respond(i18n("lastfm-username-saved"), &telegram.SendOptions{
		ParseMode: telegram.HTML,
		ReplyID:   resp.ID,
	})

	return err
}

func musicHandler(message *telegram.NewMessage) error {
	_, err := message.Reply(lastfm(message, "track"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		InvertMedia: true,
		LinkPreview: true,
	})
	return err
}

func albumHandler(message *telegram.NewMessage) error {
	_, err := message.Reply(lastfm(message, "album"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		InvertMedia: true,
		LinkPreview: true,
	})
	return err
}

func artistHandler(message *telegram.NewMessage) error {
	_, err := message.Reply(lastfm(message, "artist"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		InvertMedia: true,
		LinkPreview: true,
	})
	return err
}

func LastfmInline(m *telegram.InlineSend, methodType string) error {
	i18n := localization.Get(m)

	lastFMUsername, err := getUserLastFMUsername(m.SenderID)
	if err != nil || lastFMUsername == "" {
		_, err := m.Edit(i18n("lastfm-username-not-found-inline"), &telegram.SendOptions{
			ParseMode: telegram.HTML,
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(
				telegram.ButtonBuilder{}.Row(
					telegram.ButtonBuilder{}.URL(
						i18n("start-button"),
						fmt.Sprintf("https://t.me/%s?start=setuser", m.Client.Me().Username),
					),
				),
			),
		})
		return err
	}
	_, err = m.Edit(lastfm(m, methodType), &telegram.SendOptions{
		ParseMode:   telegram.HTML,
		LinkPreview: true,
		InvertMedia: true,
	})
	return err
}

func lastfm(update any, methodType string) string {
	i18n := localization.Get(update)

	var sender *telegram.UserObj
	switch u := update.(type) {
	case *telegram.NewMessage:
		sender = u.Sender
	case *telegram.InlineQuery:
		sender = u.Sender
	case *telegram.InlineSend:
		sender = u.Sender
	}

	lastFMUsername, err := getUserLastFMUsername(sender.ID)
	if err != nil {
		return i18n("lastfm-username-not-found")
	}

	recentTracks, err := lastFM.GetRecentTrack(methodType, lastFMUsername)
	if err != nil {
		return getErrorMessage(err, i18n)
	}

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)
	text += i18n("lastfm-playing", map[string]any{
		"nowplaying":     fmt.Sprintf("%v", recentTracks.Nowplaying),
		"lastFMUsername": lastFMUsername,
		"firstName":      sender.FirstName,
		"playcount":      recentTracks.Playcount,
	})

	switch methodType {
	case "track":
		text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Track)
		if recentTracks.Trackloved {
			text += " ❤️"
		}
	case "album":
		text += fmt.Sprintf("\n\n🎙<b>%s</b>", recentTracks.Artist)
	case "artist":
		text += fmt.Sprintf("\n\n<b>%s</b> - %s", recentTracks.Artist, recentTracks.Album)

	}

	return text
}

func Load(client *telegram.Client) {
	utils.SotreHelp("lastfm")
	client.On("command:setuser", handlers.HandleCommand(SetUserHandler))
	client.On("command:lastfm", handlers.HandleCommand(musicHandler))
	client.On("command:lt", handlers.HandleCommand(musicHandler))
	client.On("command:lmu", handlers.HandleCommand(musicHandler))
	client.On("command:album", handlers.HandleCommand(albumHandler))
	client.On("command:lalb", handlers.HandleCommand(albumHandler))
	client.On("command:alb", handlers.HandleCommand(albumHandler))
	client.On("command:artist", handlers.HandleCommand(artistHandler))
	client.On("command:lart", handlers.HandleCommand(artistHandler))
	client.On("command:art", handlers.HandleCommand(artistHandler))

	handlers.DisableableCommands = append(handlers.DisableableCommands,
		"setuser", "lastfm", "lt", "lmu", "album", "lalb", "alb", "artist", "lart", "art")
}
