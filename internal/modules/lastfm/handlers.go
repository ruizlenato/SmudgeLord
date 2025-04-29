package lastfm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var lastFM = lastFMAPI.Init()

func setUserHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	message := update.Message

	if message.Chat.Type == models.ChatTypeGroup || message.Chat.Type == models.ChatTypeSupergroup && message.From.ID == message.Chat.ID {
		return
	}

	i18n := localization.Get(update)
	var lastFMUsername string

	if len(strings.Fields(message.Text)) > 1 {
		lastFMUsername = strings.Fields(message.Text)[1]
	} else {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("no-lastfm-username-provided"),
			ParseMode: "HTML",
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if lastFM.GetUser(lastFMUsername) != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("invalid-lastfm-username"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if err := setLastFMUsername(message.From.ID, lastFMUsername); err != nil {
		slog.Error("Couldn't set LastFM username",
			"UserID", message.From.ID,
			"Username", lastFMUsername,
			"Error", err.Error())
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("lastfm-username-saved"),
		ParseMode: "HTML",
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

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

func musicHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	lastfm(ctx, b, update, "track")
}

func albmHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	lastfm(ctx, b, update, "album")
}

func artistHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	lastfm(ctx, b, update, "artist")
}

func lastfm(ctx context.Context, b *bot.Bot, update *models.Update, methodType string) {
	i18n := localization.Get(update)
	lastFMUsername, err := getUserLastFMUsername(update.Message.From.ID)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("lastfm-username-not-defined"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	recentTracks, err := lastFM.GetRecentTrack(methodType, lastFMUsername)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      getErrorMessage(err, i18n),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	text := fmt.Sprintf("<a href='%s'>\u200c</a>", recentTracks.Image)
	text += i18n("lastfm-playing", map[string]interface{}{
		"nowplaying":     fmt.Sprintf("%v", recentTracks.Nowplaying),
		"lastFMUsername": lastFMUsername,
		"firstName":      update.Message.From.FirstName,
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
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			PreferLargeMedia: bot.True(),
			ShowAboveText:    bot.True(),
		},
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "setuser", bot.MatchTypeCommand, setUserHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "lastfm", bot.MatchTypeCommand, musicHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "lmu", bot.MatchTypeCommand, musicHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "lt", bot.MatchTypeCommand, musicHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "np", bot.MatchTypeCommand, musicHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "album", bot.MatchTypeCommand, albmHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "alb", bot.MatchTypeCommand, albmHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "lalb", bot.MatchTypeCommand, albmHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "artist", bot.MatchTypeCommand, artistHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "art", bot.MatchTypeCommand, artistHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "lart", bot.MatchTypeCommand, artistHandler)

	utils.SaveHelp("lastfm")
	utils.DisableableCommands = append(utils.DisableableCommands,
		"lastfm", "lmu", "lt", "np", "album", "lalb", "alb", "artist", "lart", "art")
}
