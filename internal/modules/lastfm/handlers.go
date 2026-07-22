// Package lastfm implements Last.fm features including scrobbling and collages.
package lastfm

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	callbackquery "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/conversation"
)

var lastFM = lastFMAPI.Init()
var convManager *conversation.Manager
var convDispatcher *ext.Dispatcher
var convOnce sync.Once

var collageBuildState = struct {
	mu       sync.Mutex
	building map[int64]bool
}{
	building: make(map[int64]bool),
}

func tryStartCollageBuild(userID int64) bool {
	collageBuildState.mu.Lock()
	defer collageBuildState.mu.Unlock()

	if collageBuildState.building[userID] {
		return false
	}

	collageBuildState.building[userID] = true
	return true
}

func finishCollageBuild(userID int64) {
	collageBuildState.mu.Lock()
	delete(collageBuildState.building, userID)
	collageBuildState.mu.Unlock()
}

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
	defer conv.End()

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
		_, _ = b.SendMessage(chatID, i18n("didnt-replied-with-lastfm-username"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: msgAsk.GetMessageId()},
		})
		return nil
	}

	username := strings.TrimPrefix(strings.TrimSpace(msgAsk.GetText()), "@")
	if username == "" {
		return nil
	}

	if err := lastFM.GetUser(username); err != nil {
		_, _ = b.SendMessage(chatID, i18n("invalid-lastfm-username"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}

	if err := setLastFMUsername(userID, username); err != nil {
		slog.Error("Couldn't set LastFM username", "UserID", userID, "Username", username, "Error", err.Error())
		return nil
	}
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

func collageHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	if !tryStartCollageBuild(ctx.EffectiveUser.Id) {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("lastfm-collage-busy"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}
	defer finishCollageBuild(ctx.EffectiveUser.Id)

	lastFMUsername, err := getUserLastFMUsername(ctx.EffectiveUser.Id)
	if err != nil || lastFMUsername == "" {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("lastfm-username-not-found"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}

	collageType := "album"
	period := "7day"
	gridSize := 3

	args := strings.Fields(strings.TrimSpace(ctx.EffectiveMessage.Text))
	typeAlbumAliases := parseAliasList(i18n("lastfm-collage-type-album-aliases"), []string{"album", "albums"})
	typeArtistAliases := parseAliasList(i18n("lastfm-collage-type-artist-aliases"), []string{"artist", "artists"})
	typeTrackAliases := parseAliasList(i18n("lastfm-collage-type-track-aliases"), []string{"track", "tracks"})
	period7dayAliases := parseAliasList(i18n("lastfm-collage-period-7day-aliases"), []string{"7day", "7d", "1w"})
	period1monthAliases := parseAliasList(i18n("lastfm-collage-period-1month-aliases"), []string{"1month", "1m"})
	period3monthAliases := parseAliasList(i18n("lastfm-collage-period-3month-aliases"), []string{"3month", "3m"})
	period6monthAliases := parseAliasList(i18n("lastfm-collage-period-6month-aliases"), []string{"6month", "6m"})
	period12monthAliases := parseAliasList(i18n("lastfm-collage-period-12month-aliases"), []string{"12month", "12m", "1y"})
	periodOverallAliases := parseAliasList(i18n("lastfm-collage-period-overall-aliases"), []string{"overall", "all"})
	periodUnitDayAliases := parseAliasList(i18n("lastfm-collage-period-unit-day-aliases"), []string{"d"})
	periodUnitWeekAliases := parseAliasList(i18n("lastfm-collage-period-unit-week-aliases"), []string{"w"})
	periodUnitMonthAliases := parseAliasList(i18n("lastfm-collage-period-unit-month-aliases"), []string{"m"})
	periodUnitYearAliases := parseAliasList(i18n("lastfm-collage-period-unit-year-aliases"), []string{"y"})

	if len(args) > 1 {
		for _, raw := range args[1:] {
			tok := strings.ToLower(strings.TrimSpace(raw))

			switch tok {
			case "album", "albums":
				collageType = "album"
				continue
			case "artist", "artists":
				collageType = "artist"
				continue
			case "track", "tracks":
				collageType = "track"
				continue
			case "7day", "7d", "1w":
				period = "7day"
				continue
			case "1month", "1m":
				period = "1month"
				continue
			case "3month", "3m":
				period = "3month"
				continue
			case "6month", "6m":
				period = "6month"
				continue
			case "12month", "12m", "1y":
				period = "12month"
				continue
			case "overall", "all":
				period = "overall"
				continue
			}

			if typeAlbumAliases[tok] {
				collageType = "album"
				continue
			}
			if typeArtistAliases[tok] {
				collageType = "artist"
				continue
			}
			if typeTrackAliases[tok] {
				collageType = "track"
				continue
			}
			if period7dayAliases[tok] {
				period = "7day"
				continue
			}
			if period1monthAliases[tok] {
				period = "1month"
				continue
			}
			if period3monthAliases[tok] {
				period = "3month"
				continue
			}
			if period6monthAliases[tok] {
				period = "6month"
				continue
			}
			if period12monthAliases[tok] {
				period = "12month"
				continue
			}
			if periodOverallAliases[tok] {
				period = "overall"
				continue
			}
			if mapped, ok := parseFlexiblePeriodToken(tok, periodUnitDayAliases, periodUnitWeekAliases, periodUnitMonthAliases, periodUnitYearAliases); ok {
				period = mapped
				continue
			}

			if strings.Contains(tok, "x") {
				parts := strings.SplitN(tok, "x", 2)
				if len(parts) == 2 {
					if n1, err1 := strconv.Atoi(parts[0]); err1 == nil {
						if n2, err2 := strconv.Atoi(parts[1]); err2 == nil && n2 == n1 {
							gridSize = n1
						}
					}
				}
				continue
			}

			if n, err := strconv.Atoi(tok); err == nil {
				gridSize = n
			}
		}

		if gridSize < 2 {
			gridSize = 2
		}
		if gridSize > 8 {
			gridSize = 8
		}
	}

	withText := true
	loadingMsg, _ := b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("lastfm-collage-generating"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	})

	cacheKey := collageResultCacheKey(lastFMUsername, collageType, period, gridSize, withText)
	collageBytes, err := cache.GetCacheBytes(cacheKey)
	if err != nil || len(collageBytes) == 0 {
		collageBytes, err = buildLastFMCollage(collageType, period, lastFMUsername, gridSize, withText)
		if err == nil && len(collageBytes) > 0 {
			_ = cache.SetCacheBytes(cacheKey, collageBytes, 5*time.Minute)
		}
	}
	if err != nil {
		if loadingMsg != nil {
			_, _ = b.DeleteMessage(ctx.EffectiveMessage.Chat.Id, loadingMsg.MessageId, nil)
		}
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("lastfm-collage-error"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
			ReplyMarkup:     utils.ErrorReportKeyboard(i18n),
		})
		return nil
	}

	caption := formatCollageCaption(ctx.EffectiveUser.FirstName, ctx.EffectiveUser.Username, lastFMUsername, gridSize, collageType, period)

	_, err = b.SendPhotoWithContext(context.Background(), ctx.EffectiveMessage.Chat.Id, gotgbot.InputFileByReader("lastfm-collage.jpg", bytes.NewReader(collageBytes)), &gotgbot.SendPhotoOpts{
		Caption:         caption,
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup:     buildCollageKeyboard(ctx.EffectiveUser.Id, collageType, compactPeriodForCallback(period), gridSize, withText),
	})
	if loadingMsg != nil {
		_, _ = b.DeleteMessage(ctx.EffectiveMessage.Chat.Id, loadingMsg.MessageId, nil)
	}
	if err != nil {
		slog.Error("Couldn't send collage", "error", err.Error())
	}

	return nil
}

func collageCallbackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	if !tryStartCollageBuild(ctx.CallbackQuery.From.Id) {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{
			Text:      i18n("lastfm-collage-busy"),
			ShowAlert: true,
		})
		return nil
	}
	defer finishCollageBuild(ctx.CallbackQuery.From.Id)

	parts := strings.Split(ctx.CallbackQuery.Data, "|")
	if len(parts) != 7 {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
		return nil
	}

	ownerID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
		return nil
	}
	if ownerID != ctx.CallbackQuery.From.Id {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("denied-button-alert"), ShowAlert: true})
		return nil
	}

	collageType := parts[1]
	period := parts[2]
	periodUnitDayAliases := parseAliasList(i18n("lastfm-collage-period-unit-day-aliases"), []string{"d"})
	periodUnitWeekAliases := parseAliasList(i18n("lastfm-collage-period-unit-week-aliases"), []string{"w"})
	periodUnitMonthAliases := parseAliasList(i18n("lastfm-collage-period-unit-month-aliases"), []string{"m"})
	periodUnitYearAliases := parseAliasList(i18n("lastfm-collage-period-unit-year-aliases"), []string{"y"})

	if strings.HasPrefix(period, "rel:") {
		token := strings.TrimSpace(strings.TrimPrefix(period, "rel:"))
		mapped, ok := parseFlexiblePeriodToken(token, periodUnitDayAliases, periodUnitWeekAliases, periodUnitMonthAliases, periodUnitYearAliases)
		if !ok {
			_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("lastfm-collage-error-short"), ShowAlert: true})
			return nil
		}
		period = mapped
	}

	gridSize, err := strconv.Atoi(parts[4])
	if err != nil {
		gridSize = 3
	}
	withText := parts[5] == "1"
	action := parts[6]

	switch action {
	case "plus":
		gridSize++
	case "minus":
		gridSize--
	case "text":
		withText = !withText
	}

	if gridSize < 2 {
		gridSize = 2
	}
	if gridSize > 8 {
		gridSize = 8
	}

	username, err := getUserLastFMUsername(ownerID)
	if err != nil || username == "" {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("lastfm-username-not-found"), ShowAlert: true})
		return nil
	}

	cacheKey := collageResultCacheKey(username, collageType, period, gridSize, withText)
	collageBytes, err := cache.GetCacheBytes(cacheKey)
	if err != nil || len(collageBytes) == 0 {
		collageBytes, err = buildLastFMCollage(collageType, period, username, gridSize, withText)
		if err == nil && len(collageBytes) > 0 {
			_ = cache.SetCacheBytes(cacheKey, collageBytes, 5*time.Minute)
		}
	}
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("lastfm-collage-error-short"), ShowAlert: true})
		return nil
	}

	caption := formatCollageCaption(ctx.CallbackQuery.From.FirstName, ctx.CallbackQuery.From.Username, username, gridSize, collageType, period)
	_, _, err = b.EditMessageMedia(gotgbot.InputMediaPhoto{
		Media:     gotgbot.InputFileByReader("lastfm-collage.jpg", bytes.NewReader(collageBytes)),
		Caption:   caption,
		ParseMode: gotgbot.ParseModeHTML,
	}, &gotgbot.EditMessageMediaOpts{
		ChatId:      ctx.CallbackQuery.Message.GetChat().Id,
		MessageId:   ctx.CallbackQuery.Message.GetMessageId(),
		ReplyMarkup: buildCollageKeyboard(ownerID, collageType, compactPeriodForCallback(period), gridSize, withText),
	})
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("lastfm-collage-error-short"), ShowAlert: true})
		return nil
	}

	_, _, _ = b.EditMessageCaption(&gotgbot.EditMessageCaptionOpts{
		ChatId:      ctx.CallbackQuery.Message.GetChat().Id,
		MessageId:   ctx.CallbackQuery.Message.GetMessageId(),
		Caption:     caption,
		ParseMode:   gotgbot.ParseModeHTML,
		ReplyMarkup: buildCollageKeyboard(ownerID, collageType, compactPeriodForCallback(period), gridSize, withText),
	})

	_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
	return nil
}

func buildCollageKeyboard(userID int64, currentType, currentPeriod string, gridSize int, withText bool) gotgbot.InlineKeyboardMarkup {
	cb := func(action string) string {
		textFlag := "0"
		if withText {
			textFlag = "1"
		}
		return fmt.Sprintf("lfmcol|%s|%s|%d|%d|%s|%s", currentType, currentPeriod, userID, gridSize, textFlag, action)
	}

	plusBtn := gotgbot.InlineKeyboardButton{Text: "➕", CallbackData: cb("plus")}
	minusBtn := gotgbot.InlineKeyboardButton{Text: "➖", CallbackData: cb("minus")}
	if gridSize >= 8 {
		plusBtn.Style = gotgbot.KeyboardButtonStyleDanger
	}
	if gridSize <= 2 {
		minusBtn.Style = gotgbot.KeyboardButtonStyleDanger
	}

	toggleLabel := "🧼"
	if !withText {
		toggleLabel = "📝"
	}

	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
		{
			plusBtn,
			minusBtn,
			{Text: toggleLabel, CallbackData: cb("text")},
		},
	}}
}

func parseAliasList(raw string, defaults []string) map[string]bool {
	aliases := make(map[string]bool, len(defaults)+4)
	for _, d := range defaults {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			aliases[d] = true
		}
	}

	for _, part := range strings.Split(raw, "|") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p != "" {
			aliases[p] = true
		}
	}

	return aliases
}

func parseFlexiblePeriodToken(tok string, dayAliases, weekAliases, monthAliases, yearAliases map[string]bool) (string, bool) {
	tok = strings.TrimSpace(strings.ToLower(tok))
	if tok == "" {
		return "", false
	}

	m := periodTokenPattern.FindStringSubmatch(tok)
	if len(m) != 3 {
		return "", false
	}

	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return "", false
	}

	days := 0
	unit := m[2]
	switch {
	case dayAliases[unit]:
		days = n
	case weekAliases[unit]:
		days = n * 7
	case monthAliases[unit]:
		days = n * 30
	case yearAliases[unit]:
		days = n * 365
	default:
		return "", false
	}
	if days <= 0 {
		return "", false
	}

	to := time.Now().Unix()
	from := to - int64(days*24*60*60)
	if from <= 0 || to <= from {
		return "", false
	}

	return fmt.Sprintf("range:%d:%d:%s", from, to, tok), true
}

var periodTokenPattern = regexp.MustCompile(`^(\d+)\s*([\p{L}]+)$`)

func formatCollageCaption(firstName, telegramUsername, lastFMUsername string, gridSize int, collageType, period string) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		name = lastFMUsername
	}

	periodShort := map[string]string{
		"7day":    "1w",
		"1month":  "1m",
		"3month":  "3m",
		"6month":  "6m",
		"12month": "1y",
		"overall": "all",
	}[period]
	if strings.HasPrefix(period, "range:") {
		parts := strings.Split(period, ":")
		if len(parts) == 4 && strings.TrimSpace(parts[3]) != "" {
			periodShort = parts[3]
		}
	}
	if periodShort == "" {
		periodShort = period
	}

	return fmt.Sprintf("<a href='https://last.fm/user/%s'>%s</a>\n%dx%d, %s, %s",
		html.EscapeString(lastFMUsername),
		html.EscapeString(name),
		gridSize,
		gridSize,
		html.EscapeString(collageType),
		html.EscapeString(periodShort),
	)
}

func collageResultCacheKey(lastFMUsername, collageType, period string, gridSize int, withText bool) string {
	input := fmt.Sprintf("%s|%s|%s|%d|%t", strings.ToLower(strings.TrimSpace(lastFMUsername)), collageType, period, gridSize, withText)
	h := sha1.Sum([]byte(input))
	return "lastfm:collage:" + hex.EncodeToString(h[:])
}

func compactPeriodForCallback(period string) string {
	if strings.HasPrefix(period, "range:") {
		parts := strings.Split(period, ":")
		if len(parts) == 4 {
			token := strings.TrimSpace(parts[3])
			if token != "" {
				return "rel:" + token
			}
		}
	}
	return period
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
	dispatcher.AddHandler(utils.NewDisableableCommand("collage", collageHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("lfmcol"), collageCallbackHandler))

	utils.SaveHelp("lastfm")
}
