package lastfm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/choseninlineresult"

	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/conversation"
)

var lastFM = lastFMAPI.Init()
var convManager *conversation.Manager
var convDispatcher *ext.Dispatcher

func setUserHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}

	if convManager == nil {
		if convDispatcher == nil {
			return nil
		}
		convManager = conversation.NewManager(b, convDispatcher)
	}

	if convManager == nil {
		return nil
	}

	chatID := ctx.EffectiveMessage.Chat.Id
	userID := ctx.EffectiveUser.Id
	i18n := localization.Get(ctx)

	conv := convManager.Start(chatID, userID, &conversation.ConversationOptions{Timeout: 5 * time.Minute})

	msgAsk, err := conv.Ask(context.Background(), i18n("reply-with-lastfm-username"), &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId: ctx.EffectiveMessage.MessageId,
		},
		ReplyMarkup: gotgbot.ForceReply{
			ForceReply: true,
			Selective:  true,
		},
	})
	if err != nil {
		if err == conversation.ErrConversationTimeout {
			return nil
		}
		slog.Error("Error while asking for LastFM username", "error", err.Error())
		return nil
	}

	if msgAsk.ReplyToMessage == nil || msgAsk.ReplyToMessage.GetMessageId() != conv.GetLastMessageID() {
		conv.End()
		_, _ = b.SendMessage(chatID, i18n("didnt-replied-with-lastfm-username"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: msgAsk.GetMessageId()},
		})
		return nil
	}

	username := strings.TrimPrefix(strings.TrimSpace(msgAsk.GetText()), "@")
	if username == "" {
		conv.End()
		return nil
	}

	if err := lastFM.GetUser(username); err != nil {
		_, _ = b.SendMessage(chatID, i18n("invalid-lastfm-username"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		conv.End()
		return nil
	}

	if err := setLastFMUsername(userID, username); err != nil {
		slog.Error("Couldn't set LastFM username", "UserID", userID, "Username", username, "Error", err.Error())
		conv.End()
		return nil
	}

	conv.End()
	_, _ = b.SendMessage(chatID, i18n("lastfm-username-saved"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	})

	return nil
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

func musicHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	return sendLastfmMessage(b, ctx, "track")
}

func albmHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	return sendLastfmMessage(b, ctx, "album")
}

func artistHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	return sendLastfmMessage(b, ctx, "artist")
}

func sendLastfmMessage(b *gotgbot.Bot, ctx *ext.Context, methodType string) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, lastfm(ctx, methodType), &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			PreferLargeMedia: true,
			ShowAboveText:    true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	})

	return nil
}

func LastfmInline(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.ChosenInlineResult == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	lastFMUsername, err := getUserLastFMUsername(ctx.ChosenInlineResult.From.Id)
	if err != nil || lastFMUsername == "" {
		_, _, _ = b.EditMessageText(i18n("lastfm-username-not-found-inline"), &gotgbot.EditMessageTextOpts{
			InlineMessageId: ctx.ChosenInlineResult.InlineMessageId,
			ParseMode:       gotgbot.ParseModeHTML,
		})
		return nil
	}

	_, _, _ = b.EditMessageText(lastfm(ctx, ctx.ChosenInlineResult.ResultId), &gotgbot.EditMessageTextOpts{
		InlineMessageId: ctx.ChosenInlineResult.InlineMessageId,
		ParseMode:       gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			PreferLargeMedia: true,
			ShowAboveText:    true,
		},
	})

	return nil
}

func lastfm(ctx *ext.Context, methodType string) string {
	i18n := localization.Get(ctx)
	if ctx.EffectiveUser == nil {
		return i18n("lastfm-username-not-found")
	}

	lastFMUsername, err := getUserLastFMUsername(ctx.EffectiveUser.Id)
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
		"firstName":      ctx.EffectiveUser.FirstName,
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

	return text
}

func Load(dispatcher *ext.Dispatcher) {
	convDispatcher = dispatcher

	dispatcher.AddHandler(handlers.NewCommand("setuser", setUserHandler))
	dispatcher.AddHandler(handlers.NewCommand("lastfm", musicHandler))
	dispatcher.AddHandler(handlers.NewCommand("lmu", musicHandler))
	dispatcher.AddHandler(handlers.NewCommand("lt", musicHandler))
	dispatcher.AddHandler(handlers.NewCommand("np", musicHandler))
	dispatcher.AddHandler(handlers.NewCommand("album", albmHandler))
	dispatcher.AddHandler(handlers.NewCommand("alb", albmHandler))
	dispatcher.AddHandler(handlers.NewCommand("lalb", albmHandler))
	dispatcher.AddHandler(handlers.NewCommand("artist", artistHandler))
	dispatcher.AddHandler(handlers.NewCommand("art", artistHandler))
	dispatcher.AddHandler(handlers.NewCommand("lart", artistHandler))
	dispatcher.AddHandler(handlers.NewChosenInlineResult(choseninlineresult.All, LastfmInline))

	utils.SaveHelp("lastfm")
	utils.DisableableCommands = append(utils.DisableableCommands,
		"lastfm", "lmu", "lt", "np", "album", "lalb", "alb", "artist", "lart", "art")
}
