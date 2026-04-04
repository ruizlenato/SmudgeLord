package stickers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/conversation"
)

var (
	convDispatcher *ext.Dispatcher
	convManager    *conversation.Manager
)

func ensureConversationManager(b *gotgbot.Bot) *conversation.Manager {
	if convManager == nil && convDispatcher != nil {
		convManager = conversation.NewManager(b, convDispatcher)
	}
	return convManager
}

func kangErrorMessage(i18n func(string, ...map[string]any) string, userID int64) (string, string) {
	errorID := utils.NewUserErrorID(userID)
	return utils.BuildErrorReportMessage(i18n, "kang-error-summary", errorID), errorID
}

func sendKangErrorMessage(b *gotgbot.Bot, chatID int64, replyTo int64, userID int64, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	text, errorID := kangErrorMessage(i18n, userID)
	if logMsg != "" {
		utils.LogErrorWithID(logMsg, errorID, err, "userID", userID, "chatID", chatID)
	}

	opts := &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML}
	opts.ReplyMarkup = utils.ErrorReportKeyboard(i18n)
	if replyTo > 0 {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: replyTo}
	}
	_, _ = b.SendMessage(chatID, text, opts)
}

func answerKangErrorCallback(b *gotgbot.Bot, callbackID string, userID int64, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	errorID := utils.NewUserErrorID(userID)
	if logMsg != "" {
		utils.LogErrorWithID(logMsg, errorID, err, "userID", userID)
	}
	_, _ = b.AnswerCallbackQuery(callbackID, &gotgbot.AnswerCallbackQueryOpts{Text: utils.BuildErrorReportAlert(i18n, "kang-error-summary", errorID), ShowAlert: true})
}

func sendMigrationNotice(b *gotgbot.Bot, chatID int64, replyID int, i18n func(string, ...map[string]any) string) {
	opts := &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML}
	if replyID > 0 {
		opts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: int64(replyID)}
	}
	_, _ = b.SendMessage(chatID, i18n("stickers-migration-notice"), opts)
}

func migrationPlaceholder(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	sendMigrationNotice(b, ctx.EffectiveMessage.Chat.Id, int(ctx.EffectiveMessage.MessageId), i18n)
	return nil
}

func handleConversationError(b *gotgbot.Bot, ctx *ext.Context, err error, i18n func(string, ...map[string]any) string) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	msg := i18n("stickers-newpack-timeout")
	switch {
	case errors.Is(err, conversation.ErrConversationAborted), errors.Is(err, conversation.ErrConversationCanceled):
		msg = i18n("stickers-newpack-canceled")
	case errors.Is(err, conversation.ErrConversationTimeout):
		msg = i18n("stickers-newpack-timeout")
	default:
		slog.Warn("sticker conversation failed", "error", err.Error())
		msg = i18n("stickers-migration-notice")
	}
	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, msg, &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId: ctx.EffectiveMessage.MessageId,
		},
	})
	return nil
}

func newPackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	if ctx.EffectiveMessage.ReplyToMessage == nil {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("kang-no-reply-provided"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}

	stickerAction, stickerType, fileID := getFileIDAndType(ctx.EffectiveMessage.ReplyToMessage)
	if stickerType == "" {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("sticker-invalid-media-type"), &gotgbot.SendMessageOpts{
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		})
		return nil
	}

	if err := syncUserPacks(b, ctx.EffectiveUser.Id); err != nil {
		slog.Debug("Failed to sync user packs", "error", err)
	}

	count, err := getUserPacksCount(ctx.EffectiveUser.Id)
	if err != nil {
		sendKangErrorMessage(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveMessage.MessageId, ctx.EffectiveUser.Id, i18n, "Couldn't get user packs count", err)
		return nil
	}
	if count >= maxPacks {
		_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("sticker-max-packs-reached", map[string]any{"maxPacks": maxPacks}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}

	manager := ensureConversationManager(b)
	if manager == nil {
		sendMigrationNotice(b, ctx.EffectiveMessage.Chat.Id, int(ctx.EffectiveMessage.MessageId), i18n)
		return nil
	}
	conv := manager.Start(ctx.EffectiveMessage.Chat.Id, ctx.EffectiveUser.Id, &conversation.ConversationOptions{
		Timeout:       3 * time.Minute,
		AbortKeywords: []string{"cancel", "abort", "stop"},
	})
	if conv == nil {
		sendMigrationNotice(b, ctx.EffectiveMessage.Chat.Id, int(ctx.EffectiveMessage.MessageId), i18n)
		return nil
	}
	defer conv.End()

	titleMsg, err := conv.Ask(context.Background(), i18n("stickers-newpack-title-request"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup:     gotgbot.ForceReply{ForceReply: true, Selective: true},
	})
	if err != nil {
		return handleConversationError(b, ctx, err, i18n)
	}
	if titleMsg == nil {
		return handleConversationError(b, ctx, errors.New("no title response"), i18n)
	}

	skipToken := fmt.Sprintf("newpackSkip:%d:%d", ctx.EffectiveUser.Id, time.Now().UnixNano())
	emojiPrompt, err := b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("stickers-newpack-emoji-request"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: titleMsg.MessageId},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: i18n("stickers-newpack-skip-button"), CallbackData: skipToken},
		}}},
	})
	if err != nil {
		return handleConversationError(b, ctx, err, i18n)
	}

	packEmoji, err := awaitEmojiOrSkip(b, ctx, emojiPrompt.MessageId, skipToken)
	if err != nil {
		return handleConversationError(b, ctx, err, i18n)
	}

	packTitle := strings.TrimSpace(titleMsg.GetText())
	if packTitle == "" {
		packTitle = i18n("stickers-newpack-default-title")
	}
	if packEmoji == "" || strings.EqualFold(packEmoji, "skip") || strings.EqualFold(packEmoji, "pular") || strings.EqualFold(packEmoji, i18n("stickers-newpack-skip-button")) {
		packEmoji = "🤔"
	}
	packName, err := generateStickerSetName(b, ctx.EffectiveUser.Id, count)
	if err != nil {
		sendKangErrorMessage(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveMessage.MessageId, ctx.EffectiveUser.Id, i18n, "Couldn't generate sticker set name", err)
		return nil
	}

	uploadedFileID, err := prepareStickerUpload(b, ctx.EffectiveUser.Id, fileID, stickerType, stickerAction)
	if err != nil {
		sendKangErrorMessage(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveMessage.MessageId, ctx.EffectiveUser.Id, i18n, "Couldn't upload sticker file", err)
		return nil
	}

	_, err = b.CreateNewStickerSet(ctx.EffectiveUser.Id, packName, packTitle, []gotgbot.InputSticker{{
		Sticker:   gotgbot.InputFileByID(uploadedFileID),
		Format:    stickerType,
		EmojiList: []string{packEmoji},
	}}, nil)
	if err != nil {
		sendKangErrorMessage(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveMessage.MessageId, ctx.EffectiveUser.Id, i18n, "Couldn't create sticker set", err)
		return nil
	}

	if err := createPack(ctx.EffectiveUser.Id, packName); err != nil {
		slog.Error("Couldn't save pack to database", "error", err)
	}

	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("stickers-newpack-success", map[string]any{
		"packTitle": utils.SanitizeTelegramHTML(packTitle),
		"packEmoji": utils.EscapeHTML(packEmoji),
	}), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
			Text: i18n("sticker-newpack-button"),
			Url:  "https://t.me/addstickers/" + packName,
		}}}},
	})
	return nil
}

func awaitEmojiOrSkip(b *gotgbot.Bot, ctx *ext.Context, replyToMessageID int64, skipToken string) (string, error) {
	if convDispatcher == nil || ctx.EffectiveUser == nil || ctx.EffectiveChat == nil {
		return "", errors.New("conversation unavailable")
	}

	chatID := ctx.EffectiveChat.Id
	userID := ctx.EffectiveUser.Id
	msgCh := make(chan string, 1)
	skipCh := make(chan struct{}, 1)

	msgHandler := handlers.NewMessage(func(m *gotgbot.Message) bool {
		return m != nil && m.Chat.Id == chatID && m.From != nil && m.From.Id == userID && m.ReplyToMessage != nil && m.ReplyToMessage.MessageId == replyToMessageID
	}, func(_ *gotgbot.Bot, ectx *ext.Context) error {
		if ectx.EffectiveMessage == nil {
			return nil
		}
		text := strings.TrimSpace(ectx.EffectiveMessage.GetText())
		if text == "" {
			text = strings.TrimSpace(ectx.EffectiveMessage.Caption)
		}
		select {
		case msgCh <- text:
		default:
		}
		return nil
	})

	skipHandler := handlers.NewCallback(callbackquery.Equal(skipToken), func(bot *gotgbot.Bot, ectx *ext.Context) error {
		if ectx.CallbackQuery == nil || ectx.CallbackQuery.From.Id != userID {
			return nil
		}
		if ectx.CallbackQuery.Message != nil {
			chat := ectx.CallbackQuery.Message.GetChat()
			msgID := ectx.CallbackQuery.Message.GetMessageId()
			_, _ = bot.DeleteMessage(chat.Id, msgID, nil)
		}
		_, _ = bot.AnswerCallbackQuery(ectx.CallbackQuery.Id, nil)
		select {
		case skipCh <- struct{}{}:
		default:
		}
		return nil
	})

	const group = -99
	convDispatcher.AddHandlerToGroup(msgHandler, group)
	convDispatcher.AddHandlerToGroup(skipHandler, group)
	defer convDispatcher.RemoveHandlerFromGroup(msgHandler.Name(), group)
	defer convDispatcher.RemoveHandlerFromGroup(skipHandler.Name(), group)

	timer := time.NewTimer(3 * time.Minute)
	defer timer.Stop()

	select {
	case text := <-msgCh:
		if strings.EqualFold(text, "cancel") || strings.EqualFold(text, "abort") || strings.EqualFold(text, "stop") {
			return "", conversation.ErrConversationAborted
		}
		return text, nil
	case <-skipCh:
		return "🤔", nil
	case <-timer.C:
		return "", conversation.ErrConversationTimeout
	}
}

func prepareStickerUpload(b *gotgbot.Bot, userID int64, fileID, stickerType, stickerAction string) (string, error) {
	file, err := b.GetFile(fileID, nil)
	if err != nil || file.FilePath == "" {
		return "", fmt.Errorf("get file: %w", err)
	}
	resp, err := http.Get(b.BotClient.FileURL(b.Token, file.FilePath, nil))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	switch stickerAction {
	case "resize":
		bodyBytes, err = utils.ResizeSticker(bodyBytes)
		if err != nil {
			return "", err
		}
	case "convert":
		bodyBytes, err = convertVideo(bodyBytes)
		if err != nil {
			return "", err
		}
	}
	stickerFilename := normalizeStickerFilename(filepath.Base(file.FilePath), stickerAction)
	uploaded, err := b.UploadStickerFile(userID, gotgbot.InputFileByReader(stickerFilename, bytes.NewReader(bodyBytes)), stickerType, nil)
	if err != nil {
		return "", err
	}
	return uploaded.FileId, nil
}

func validateUserPacks(b *gotgbot.Bot, userID int64) []ValidatedPack {
	packs, err := getUserPacks(userID)
	if err != nil {
		return nil
	}
	valid := make([]ValidatedPack, 0, len(packs))
	for _, pack := range packs {
		set, err := b.GetStickerSet(pack.PackName, nil)
		if err != nil || set == nil {
			continue
		}
		valid = append(valid, ValidatedPack{StickerPack: pack, Title: set.Title, StickerCount: len(set.Stickers)})
	}
	if len(valid) == 0 {
		return nil
	}
	return valid
}

func myPacksHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	if !requirePrivateChat(b, ctx, i18n) {
		return nil
	}
	_ = syncUserPacks(b, ctx.EffectiveUser.Id)
	valid := validateUserPacks(b, ctx.EffectiveUser.Id)
	if len(valid) == 0 {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-no-packs"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}
	nameTitle := getDisplayName(ctx.EffectiveUser.FirstName, ctx.EffectiveUser.Username)
	var text strings.Builder
	text.WriteString(i18n("sticker-mypacks-header", map[string]any{"userName": nameTitle}))
	text.WriteString("\n")
	for i, pack := range valid {
		mark := ""
		if pack.IsDefault {
			mark = " ✓"
		}
		fmt.Fprintf(&text, "\n%d - <a href='t.me/addstickers/%s'>%s</a>%s", i+1, pack.PackName, pack.Title, mark)
	}
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, text.String(), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true}, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func switchHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	if !requirePrivateChat(b, ctx, i18n) {
		return nil
	}
	valid := validateUserPacks(b, ctx.EffectiveUser.Id)
	if len(valid) == 0 {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-no-packs"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}
	nameTitle := getDisplayName(ctx.EffectiveUser.FirstName, ctx.EffectiveUser.Username)
	list, buttons := buildSwitchPackList(valid, ctx.EffectiveUser.Id, i18n)
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-switch-select", map[string]any{"userName": nameTitle})+"\n"+list, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true}, ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func switchPackCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	parts := strings.Split(ctx.CallbackQuery.Data, " ")
	if len(parts) != 3 {
		return nil
	}
	ownerID, _ := strconv.ParseInt(parts[1], 10, 64)
	packID, _ := strconv.ParseInt(parts[2], 10, 64)
	if ctx.CallbackQuery.From.Id != ownerID {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("denied-button-alert"), ShowAlert: true})
		return nil
	}
	if err := setDefaultPack(ownerID, packID); err != nil {
		answerKangErrorCallback(b, ctx.CallbackQuery.Id, ctx.CallbackQuery.From.Id, i18n, "Couldn't set default pack", err)
		return nil
	}
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()
	_, _, _ = b.EditMessageText(i18n("sticker-pack-switched"), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: msgID, ParseMode: gotgbot.ParseModeHTML})
	_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
	return nil
}

func delPackHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	if !requirePrivateChat(b, ctx, i18n) {
		return nil
	}
	valid := validateUserPacks(b, ctx.EffectiveUser.Id)
	if len(valid) == 0 {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-no-packs"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}
	var text strings.Builder
	text.WriteString(i18n("sticker-delpack-select"))
	text.WriteString("\n")
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(valid))
	for i, pack := range valid {
		fmt.Fprintf(&text, "\n%d - %s", i+1, pack.Title)
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{Text: fmt.Sprintf("%d", i+1), CallbackData: fmt.Sprintf("delPack %d %d", ctx.EffectiveUser.Id, pack.ID)}})
	}
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, text.String(), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func delPackCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	parts := strings.Split(ctx.CallbackQuery.Data, " ")
	if len(parts) != 3 {
		return nil
	}
	ownerID, _ := strconv.ParseInt(parts[1], 10, 64)
	packID, _ := strconv.ParseInt(parts[2], 10, 64)
	if ctx.CallbackQuery.From.Id != ownerID {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("denied-button-alert"), ShowAlert: true})
		return nil
	}
	packs, _ := getUserPacks(ownerID)
	var deletedPackName string
	for _, p := range packs {
		if p.ID == packID {
			deletedPackName = p.PackName
			break
		}
	}
	if err := deletePack(ownerID, packID); err != nil {
		answerKangErrorCallback(b, ctx.CallbackQuery.Id, ctx.CallbackQuery.From.Id, i18n, "Couldn't delete sticker pack", err)
		return nil
	}
	if deletedPackName != "" {
		_, _ = b.DeleteStickerSet(deletedPackName, nil)
	}
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()
	_, _, _ = b.EditMessageText(i18n("sticker-pack-deleted"), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: msgID, ParseMode: gotgbot.ParseModeHTML})
	_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
	return nil
}

func kangStickerHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	if ctx.EffectiveMessage.ReplyToMessage == nil {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("kang-no-reply-provided"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}
	stickerAction, stickerType, fileID := getFileIDAndType(ctx.EffectiveMessage.ReplyToMessage)
	if stickerType == "" {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-invalid-media-type"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}
	_ = syncUserPacks(b, ctx.EffectiveUser.Id)
	packs, _ := getUserPacks(ctx.EffectiveUser.Id)
	if len(packs) == 0 {
		packName, _ := generateStickerSetName(b, ctx.EffectiveUser.Id, 0)
		_ = createPack(ctx.EffectiveUser.Id, packName)
		packs, _ = getUserPacks(ctx.EffectiveUser.Id)
	}
	defaultPack, _ := getDefaultPack(ctx.EffectiveUser.Id)
	if len(packs) > 1 && defaultPack == nil {
		return showKangPackSelection(b, ctx, packs, stickerAction, stickerType, fileID)
	}
	target := defaultPack
	if target == nil && len(packs) > 0 {
		target = &packs[0]
	}
	if target == nil {
		return nil
	}
	return processKangIntoPack(b, ctx, *target, stickerAction, stickerType, fileID, extractEmojis(ctx.EffectiveMessage.GetText(), ctx.EffectiveMessage.ReplyToMessage.Sticker)[0])
}

func showKangPackSelection(b *gotgbot.Bot, ctx *ext.Context, packs []StickerPack, stickerAction, stickerType, fileID string) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	nameTitle := getDisplayName(ctx.EffectiveUser.FirstName, ctx.EffectiveUser.Username)
	var text strings.Builder
	text.WriteString(i18n("sticker-select-pack", map[string]any{"userName": nameTitle}))
	text.WriteString("\n")
	buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(packs))
	for i, pack := range packs {
		set, err := b.GetStickerSet(pack.PackName, nil)
		if err != nil || set == nil {
			continue
		}
		if len(set.Stickers) >= 120 {
			fmt.Fprintf(&text, "\nX - <a href='t.me/addstickers/%s'>%s</a> <b>%s</b>", pack.PackName, set.Title, i18n("sticker-pack-full-mark"))
			continue
		}
		fmt.Fprintf(&text, "\n%d - <a href='t.me/addstickers/%s'>%s</a>", i+1, pack.PackName, set.Title)
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{Text: fmt.Sprintf("%d", i+1), CallbackData: fmt.Sprintf("kangPack %d %d", ctx.EffectiveMessage.MessageId, pack.ID)}})
	}
	kd := KangData{StickerAction: stickerAction, StickerType: stickerType, FileID: fileID, Emoji: extractEmojis(ctx.EffectiveMessage.GetText(), ctx.EffectiveMessage.ReplyToMessage.Sticker)[0]}
	if payload, err := json.Marshal(kd); err == nil {
		_ = cache.SetCache(fmt.Sprintf("kang:%d:%d", ctx.EffectiveUser.Id, ctx.EffectiveMessage.MessageId), string(payload), 5*time.Minute)
	}
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, text.String(), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true}, ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons}, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func kangPackCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	parts := strings.Split(ctx.CallbackQuery.Data, " ")
	if len(parts) != 3 {
		return nil
	}
	msgID, _ := strconv.ParseInt(parts[1], 10, 64)
	packID, _ := strconv.ParseInt(parts[2], 10, 64)
	key := fmt.Sprintf("kang:%d:%d", ctx.CallbackQuery.From.Id, msgID)
	payload, err := cache.GetCache(key)
	if err != nil || payload == "" {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("sticker-kang-expired"), ShowAlert: true})
		return nil
	}
	var kd KangData
	if err := json.Unmarshal([]byte(payload), &kd); err != nil {
		answerKangErrorCallback(b, ctx.CallbackQuery.Id, ctx.CallbackQuery.From.Id, i18n, "Couldn't decode cached kang data", err)
		return nil
	}
	packs, _ := getUserPacks(ctx.CallbackQuery.From.Id)
	var target *StickerPack
	for _, p := range packs {
		if p.ID == packID {
			target = &p
			break
		}
	}
	if target == nil {
		answerKangErrorCallback(b, ctx.CallbackQuery.Id, ctx.CallbackQuery.From.Id, i18n, "Couldn't find selected sticker pack", nil)
		return nil
	}
	ctx.EffectiveUser = &ctx.CallbackQuery.From
	chat := ctx.CallbackQuery.Message.GetChat()
	ctx.EffectiveChat = &chat
	ctx.EffectiveMessage = &gotgbot.Message{MessageId: msgID, Chat: *ctx.EffectiveChat, From: ctx.EffectiveUser}
	_ = processKangIntoPack(b, ctx, *target, kd.StickerAction, kd.StickerType, kd.FileID, kd.Emoji)
	_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
	return nil
}

func processKangIntoPack(b *gotgbot.Bot, ctx *ext.Context, pack StickerPack, stickerAction, stickerType, fileID, emoji string) error {
	i18n := localization.Get(ctx)
	uploadedFileID, err := prepareStickerUpload(b, ctx.EffectiveUser.Id, fileID, stickerType, stickerAction)
	if err != nil {
		sendKangErrorMessage(b, ctx.EffectiveChat.Id, ctx.EffectiveMessage.MessageId, ctx.EffectiveUser.Id, i18n, "Couldn't upload sticker file", err)
		return nil
	}
	set, _ := b.GetStickerSet(pack.PackName, nil)
	if set != nil && len(set.Stickers) >= 120 {
		kd := KangData{StickerAction: stickerAction, StickerType: stickerType, FileID: uploadedFileID, Emoji: emoji}
		if payload, err := json.Marshal(kd); err == nil {
			_ = cache.SetCache(fmt.Sprintf("kangFull:%d:%d", ctx.EffectiveUser.Id, ctx.EffectiveMessage.MessageId), string(payload), 5*time.Minute)
		}
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-pack-full", map[string]any{"packName": set.Title, "stickerCount": len(set.Stickers)}), &gotgbot.SendMessageOpts{
			ParseMode: gotgbot.ParseModeHTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
				Text: i18n("sticker-create-new-pack-button"), CallbackData: fmt.Sprintf("createNewPack %d", ctx.EffectiveMessage.MessageId),
			}}}},
		})
		return nil
	}
	_, err = b.AddStickerToSet(ctx.EffectiveUser.Id, pack.PackName, gotgbot.InputSticker{Sticker: gotgbot.InputFileByID(uploadedFileID), Format: stickerType, EmojiList: []string{emoji}}, nil)
	if err != nil {
		title := generateStickerSetTitle(ctx.EffectiveUser.FirstName, ctx.EffectiveUser.Username, 0)
		_, err = b.CreateNewStickerSet(ctx.EffectiveUser.Id, pack.PackName, title, []gotgbot.InputSticker{{Sticker: gotgbot.InputFileByID(uploadedFileID), Format: stickerType, EmojiList: []string{emoji}}}, nil)
		if err != nil {
			sendKangErrorMessage(b, ctx.EffectiveChat.Id, ctx.EffectiveMessage.MessageId, ctx.EffectiveUser.Id, i18n, "Couldn't add sticker to set", err)
			return nil
		}
	}
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("sticker-stoled", map[string]any{"emoji": emoji}), &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
			Text: i18n("sticker-stoled-button"), Url: "https://t.me/addstickers/" + pack.PackName,
		}}}},
	})
	return nil
}

func createNewPackCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.Get(ctx)
	parts := strings.Split(ctx.CallbackQuery.Data, " ")
	if len(parts) != 2 {
		return nil
	}
	msgID, _ := strconv.ParseInt(parts[1], 10, 64)
	userID := ctx.CallbackQuery.From.Id
	cacheKey := fmt.Sprintf("kangFull:%d:%d", userID, msgID)
	payload, err := cache.GetCache(cacheKey)
	if err != nil || payload == "" {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("sticker-kang-expired"), ShowAlert: true})
		return nil
	}
	var kd KangData
	if err := json.Unmarshal([]byte(payload), &kd); err != nil {
		answerKangErrorCallback(b, ctx.CallbackQuery.Id, userID, i18n, "Couldn't decode full-pack kang data", err)
		return nil
	}
	count, err := getUserPacksCount(userID)
	if err != nil || count >= maxPacks {
		_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n("sticker-max-packs-reached", map[string]any{"maxPacks": maxPacks}), ShowAlert: true})
		return nil
	}
	packName, _ := generateStickerSetName(b, userID, count)
	packTitle := generateStickerSetTitle(ctx.CallbackQuery.From.FirstName, ctx.CallbackQuery.From.Username, count)
	emoji := kd.Emoji
	if emoji == "" {
		emoji = "🤔"
	}
	_, err = b.CreateNewStickerSet(userID, packName, packTitle, []gotgbot.InputSticker{{Sticker: gotgbot.InputFileByID(kd.FileID), Format: kd.StickerType, EmojiList: []string{emoji}}}, nil)
	if err != nil {
		answerKangErrorCallback(b, ctx.CallbackQuery.Id, userID, i18n, "Couldn't create sticker set from callback", err)
		return nil
	}
	_ = createPack(userID, packName)
	_ = cache.DeleteCache(cacheKey)
	chat := ctx.CallbackQuery.Message.GetChat()
	msg := ctx.CallbackQuery.Message.GetMessageId()
	_, _, _ = b.EditMessageText(i18n("sticker-pack-created", map[string]any{"packTitle": packTitle, "packNum": count + 1}), &gotgbot.EditMessageTextOpts{
		ChatId:    chat.Id,
		MessageId: msg,
		ParseMode: gotgbot.ParseModeHTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
			Text: i18n("sticker-newpack-button"), Url: "https://t.me/addstickers/" + packName,
		}}}},
	})
	_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, nil)
	return nil
}

type KangData struct {
	StickerAction string `json:"a"`
	StickerType   string `json:"t"`
	FileID        string `json:"f"`
	Emoji         string `json:"e"`
}

type ValidatedPack struct {
	StickerPack
	Title        string
	StickerCount int
}

type kangContext struct {
	chatID    int64
	msgID     int64
	userID    int64
	firstName string
	username  string
	emoji     []string
}

var emojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FAFF}]|[\x{2600}-\x{27BF}]|\x{200D}|[\x{FE00}-\x{FE0F}]|[\x{E0020}-\x{E007F}]|[\x{1F1E6}-\x{1F1FF}][\x{1F1E6}-\x{1F1FF}]`)

func getDisplayName(firstName, username string) string {
	if username != "" {
		return "@" + username
	}
	return firstName
}

func requirePrivateChat(b *gotgbot.Bot, ctx *ext.Context, i18n func(string, ...map[string]any) string) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}
	if msg.Chat.Type == gotgbot.ChatTypePrivate {
		return true
	}
	botInfo := b.User
	if botInfo.Username == "" {
		if u, err := b.GetMe(nil); err == nil {
			botInfo = *u
		}
	}
	buttonText := i18n("start-button")
	url := fmt.Sprintf("https://t.me/%s", botInfo.Username)
	_, _ = b.SendMessage(msg.Chat.Id, i18n("sticker-private-only"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
			Text: buttonText,
			Url:  url,
		}}}},
	})
	return false
}

func extractEmojis(text string, replySticker *gotgbot.Sticker) []string {
	var emoji []string
	if text != "" {
		emoji = append(emoji, emojiRegex.FindAllString(text, -1)...)
	}
	if len(emoji) == 0 && replySticker != nil {
		emoji = append(emoji, replySticker.Emoji)
	}
	if len(emoji) == 0 {
		emoji = append(emoji, "🤔")
	}
	return emoji
}

func buildSwitchPackList(packs []ValidatedPack, userID int64, i18n func(string, ...map[string]any) string) (string, [][]gotgbot.InlineKeyboardButton) {
	var text strings.Builder
	var buttons [][]gotgbot.InlineKeyboardButton
	buttonIndex := 1

	for _, pack := range packs {
		if pack.StickerCount >= 120 {
			fmt.Fprintf(&text, "\nX — %s <b>%s</b>", pack.Title, i18n("sticker-pack-full-mark"))
			continue
		}
		defaultMark := ""
		if pack.IsDefault {
			defaultMark = " ✓"
		}
		fmt.Fprintf(&text, "\n%d — %s%s", buttonIndex, pack.Title, defaultMark)
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%d", buttonIndex),
			CallbackData: fmt.Sprintf("switchPack %d %d", userID, pack.ID),
		}})
		buttonIndex++
	}

	return text.String(), buttons
}

func editErrorMessage(b *gotgbot.Bot, chatID, msgID, userID int64, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	errorID := utils.NewUserErrorID(userID)
	utils.LogErrorWithID(logMsg, errorID, err, "chatID", chatID, "userID", userID)
	_, _, _ = b.EditMessageText(utils.BuildErrorReportMessage(i18n, "kang-error-summary", errorID), &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: msgID, ParseMode: gotgbot.ParseModeHTML, ReplyMarkup: utils.ErrorReportKeyboard(i18n)})
}

func editStickerError(b *gotgbot.Bot, chatID, msgID, userID int64, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	editErrorMessage(b, chatID, msgID, userID, i18n, logMsg, err)
}

func editKangError(b *gotgbot.Bot, kc *kangContext, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	editErrorMessage(b, kc.chatID, kc.msgID, kc.userID, i18n, logMsg, err)
}

func getStickerHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.ReplyToMessage == nil || msg.ReplyToMessage.Sticker == nil {
		return nil
	}
	replySticker := msg.ReplyToMessage.Sticker
	if replySticker.IsAnimated {
		_, _ = b.SendMessage(msg.Chat.Id, localization.Get(ctx)("get-sticker-animated-not-supported"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId}})
		return nil
	}
	file, err := b.GetFile(replySticker.FileId, nil)
	if err != nil || file.FilePath == "" {
		sendKangErrorMessage(b, msg.Chat.Id, msg.MessageId, msg.From.Id, localization.Get(ctx), "Couldn't get file", err)
		return nil
	}
	resp, err := http.Get(b.BotClient.FileURL(b.Token, file.FilePath, nil))
	if err != nil {
		sendKangErrorMessage(b, msg.Chat.Id, msg.MessageId, msg.From.Id, localization.Get(ctx), "Couldn't download file", err)
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		sendKangErrorMessage(b, msg.Chat.Id, msg.MessageId, msg.From.Id, localization.Get(ctx), "Couldn't read body", err)
		return nil
	}
	filename := filepath.Base(file.FilePath)
	if replySticker.IsVideo {
		if len(filename) > 4 {
			filename = fmt.Sprintf("%s.gif", filename[:len(filename)-4])
		}
	}
	caption := fmt.Sprintf("<b>Emoji: %s</b>\n<b>ID:</b> <code>%s</code>", replySticker.Emoji, replySticker.FileId)
	_, _ = b.SendDocument(msg.Chat.Id, gotgbot.InputFileByReader(filename, bytes.NewReader(body)), &gotgbot.SendDocumentOpts{
		Caption:   caption,
		ParseMode: gotgbot.ParseModeHTML,
	})
	return nil
}

func generateStickerSetName(b *gotgbot.Bot, userID int64, packNum int) (string, error) {
	botInfo := b.User
	if botInfo.Username == "" {
		if info, err := b.GetMe(nil); err == nil {
			botInfo = *info
		}
	}
	shortNamePrefix := "a_"
	if packNum > 0 {
		shortNamePrefix = fmt.Sprintf("a_%d_", packNum)
	}
	shortNameSuffix := fmt.Sprintf("%d_by_%s", userID, botInfo.Username)
	return shortNamePrefix + shortNameSuffix, nil
}

func syncUserPacks(b *gotgbot.Bot, userID int64) error {
	botInfo := b.User
	if botInfo.Username == "" {
		if info, err := b.GetMe(nil); err == nil {
			botInfo = *info
		}
	}
	suffix := fmt.Sprintf("%d_by_%s", userID, botInfo.Username)
	packNames := make([]string, 0, maxPacks+1)
	packNames = append(packNames, "a_"+suffix)
	for i := 0; i < maxPacks; i++ {
		packNames = append(packNames, fmt.Sprintf("a_%d_%s", i, suffix))
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, packName := range packNames {
		exists, _ := packExists(packName)
		if exists {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if _, err := b.GetStickerSet(name, nil); err == nil {
				mu.Lock()
				defer mu.Unlock()
				if err := createPack(userID, name); err != nil {
					slog.Debug("Failed to save discovered pack to database", "PackName", name, "Error", err)
				} else {
					slog.Debug("Discovered and saved existing pack", "PackName", name)
				}
			}
		}(packName)
	}

	wg.Wait()
	return nil
}

func generateStickerSetTitle(firstName, username string, packNum int) string {
	nameTitle := firstName
	if username != "" {
		nameTitle = "@" + username
	}
	if len(nameTitle) > 35 {
		nameTitle = nameTitle[:35]
	}
	if packNum > 0 {
		return fmt.Sprintf("%s's SmudgeLord Pack %d", nameTitle, packNum+1)
	}
	return fmt.Sprintf("%s's SmudgeLord Pack", nameTitle)
}

func getFileIDAndType(media *gotgbot.Message) (string, string, string) {
	if media == nil {
		return "", "", ""
	}
	switch {
	case media.Document != nil:
		fileID := media.Document.FileId
		switch {
		case strings.Contains(media.Document.MimeType, "image"):
			return "resize", "static", fileID
		case strings.Contains(media.Document.MimeType, "tgsticker"):
			return "", "animated", fileID
		case strings.Contains(media.Document.MimeType, "video"):
			return "convert", "video", fileID
		}
	case len(media.Photo) > 0:
		return "resize", "static", media.Photo[len(media.Photo)-1].FileId
	case media.Video != nil:
		return "convert", "video", media.Video.FileId
	case media.Animation != nil:
		return "convert", "video", media.Animation.FileId
	}
	if media.Sticker != nil {
		sticker := media.Sticker
		if sticker.IsAnimated {
			return "", "animated", sticker.FileId
		}
		if sticker.IsVideo {
			return "", "video", sticker.FileId
		}
		return "", "static", sticker.FileId
	}
	return "", "", ""
}

func normalizeStickerFilename(filename, stickerAction string) string {
	if stickerAction == "resize" && filepath.Ext(filename) != ".webp" && filepath.Ext(filename) != ".png" {
		return strings.TrimSuffix(filename, filepath.Ext(filename)) + ".png"
	}
	return filename
}

func convertVideo(input []byte) ([]byte, error) {
	inputFile, err := os.CreateTemp("", "Smudgeinput_*.mp4")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inputFile.Name())
	if _, err := inputFile.Write(input); err != nil {
		return nil, err
	}
	tempOutput := inputFile.Name() + ".tmp.mp4"
	defer os.Remove(tempOutput)
	cmd := exec.Command("ffmpeg", "-loglevel", "quiet", "-i", inputFile.Name(), "-t", "00:00:03", "-vf", "fps=30", "-c:v", "vp9", "-b:v", "500k", "-preset", "ultrafast", "-s", "512x512", "-y", "-an", "-f", "webm", tempOutput)
	if err = cmd.Run(); err != nil {
		return nil, err
	}
	outFile, err := os.Open(tempOutput)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()
	return io.ReadAll(outFile)
}

func Load(dispatcher *ext.Dispatcher) {
	convDispatcher = dispatcher
	dispatcher.AddHandler(utils.NewDisableableCommand("getsticker", getStickerHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("kang", kangStickerHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("newpack", newPackHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("mypacks", myPacksHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("switch", switchHandler))
	dispatcher.AddHandler(utils.NewDisableableCommand("delpack", delPackHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("switchPack"), switchPackCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("delPack"), delPackCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("kangPack"), kangPackCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("createNewPack"), createNewPackCallback))

	utils.SaveHelp("stickers")
}
