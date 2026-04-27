package lastfm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/conversation"
)

var lastFM = lastFMAPI.Init()
var convManager *conversation.Manager
var convDispatcher *ext.Dispatcher
var convOnce sync.Once

func setUserHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}

	if ctx.EffectiveMessage.Chat.Type != gotgbot.ChatTypePrivate {
		i18n := localization.Get(ctx)
		opts := &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		}
		if b.User.Username != "" {
			opts.ReplyMarkup = gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
				Text: i18n("lastfm-private-start-button"),
				Url:  fmt.Sprintf("https://t.me/%s?start=setuser", b.User.Username),
			}}}}
		}
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("lastfm-private-only"), opts)
		return nil
	}

	return startSetUserConversation(b, ctx)
}

func StartSetUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return startSetUserConversation(b, ctx)
}

func startSetUserConversation(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}

	convOnce.Do(func() {
		if convDispatcher != nil {
			convManager = conversation.NewManager(b, convDispatcher)
		}
	})

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
		if errors.Is(err, conversation.ErrConversationTimeout) || errors.Is(err, conversation.ErrConversationCanceled) || errors.Is(err, conversation.ErrConversationAborted) {
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

func getErrorMessage(err error, i18n func(string, ...map[string]any) string, userID int64) string {
	switch {
	case strings.Contains(err.Error(), "no recent tracks"):
		return i18n("no-scrobbled-yet")
	case strings.Contains(err.Error(), "lastFM error"):
		errorID := utils.NewUserErrorID(userID)
		utils.LogErrorWithID("LastFM API returned an error", errorID, err, "userID", userID)
		return i18n("lastfm-error-with-id", utils.ErrorI18nArgs(errorID))
	default:
		errorID := utils.NewUserErrorID(userID)
		utils.LogErrorWithID("Failed to load LastFM data", errorID, err, "userID", userID)
		return i18n("lastfm-error-with-id", utils.ErrorI18nArgs(errorID))
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

	i18n := localization.Get(ctx)
	text, isError := lastfm(ctx, methodType)
	opts := &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			PreferLargeMedia: true,
			ShowAboveText:    true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	}
	if isError {
		opts.ReplyMarkup = utils.ErrorReportKeyboard(i18n)
	}
	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, text, opts)

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

	text, isError := lastfm(ctx, ctx.ChosenInlineResult.ResultId)
	opts := &gotgbot.EditMessageTextOpts{
		InlineMessageId: ctx.ChosenInlineResult.InlineMessageId,
		ParseMode:       gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			PreferLargeMedia: true,
			ShowAboveText:    true,
		},
	}
	if isError {
		opts.ReplyMarkup = utils.ErrorReportKeyboard(i18n)
	}
	_, _, _ = b.EditMessageText(text, opts)

	return nil
}

func lastfm(ctx *ext.Context, methodType string) (string, bool) {
	i18n := localization.Get(ctx)
	if ctx.EffectiveUser == nil {
		return i18n("lastfm-username-not-found"), false
	}

	lastFMUsername, err := getUserLastFMUsername(ctx.EffectiveUser.Id)
	if err != nil {
		return i18n("lastfm-username-not-found"), false
	}

	recentTracks, err := lastFM.GetRecentTrack(methodType, lastFMUsername)
	if err != nil {
		return getErrorMessage(err, i18n, ctx.EffectiveUser.Id), true
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

	return text, false
}

func Load(dispatcher *ext.Dispatcher) {
	convDispatcher = dispatcher

	dispatcher.AddHandler(handlers.NewCommand("setuser", setUserHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("lastfm", musicHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("lmu", musicHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("lt", musicHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("np", musicHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("album", albmHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("alb", albmHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("lalb", albmHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("artist", artistHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("art", artistHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("lart", artistHandler))

	utils.SaveHelp("lastfm")
}
