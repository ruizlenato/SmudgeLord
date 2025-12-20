package lastfm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/conversation"
)

var lastFM = lastFMAPI.Init()

func setUserHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	i18n := localization.Get(update)

	convManager := conversation.NewManager(b)
	conv := convManager.Start(chatID, userID, &conversation.ConversationOptions{
		Timeout: 5 * time.Minute,
	})

	msgAsk, err := conv.Ask(ctx, &bot.SendMessageParams{
		Text:      i18n("reply-with-lastfm-username"),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: models.ForceReply{
			ForceReply: true,
		},
	})
	if err != nil {
		if err == conversation.ErrConversationTimeout {
			return
		}
		slog.Error("Error while asking for LastFM username",
			"error", err.Error())
		return
	}

	if msgAsk.ReplyToMessage == nil || msgAsk.ReplyToMessage.ID != conv.GetLastMessageID() {
		conv.End()
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      i18n("didnt-replied-with-lastfm-username"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: msgAsk.ID,
			},
		})
		return
	}

	if lastFM.GetUser(msgAsk.Text) != nil {
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

	if err := setLastFMUsername(userID, msgAsk.Text); err != nil {
		slog.Error("Couldn't set LastFM username",
			"UserID", userID,
			"Username", msgAsk.Text,
			"Error", err.Error())
		return
	}

	conv.End()
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("lastfm-username-saved"),
		ParseMode: "HTML",
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

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
			Text:      i18n("lastfm-username-not-found"),
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
	text += i18n("lastfm-playing", map[string]any{
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
	b.RegisterHandler(bot.HandlerTypeCommand, "setuser", setUserHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "lastfm", musicHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "lmu", musicHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "lt", musicHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "np", musicHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "album", albmHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "alb", albmHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "lalb", albmHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "artist", artistHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "art", artistHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "lart", artistHandler)

	utils.SaveHelp("lastfm")
	utils.DisableableCommands = append(utils.DisableableCommands,
		"lastfm", "lmu", "lt", "np", "album", "lalb", "alb", "artist", "lart", "art")
}
