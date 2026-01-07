package stickers

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var emojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FAFF}]|[\x{2600}-\x{27BF}]|\x{200D}|[\x{FE00}-\x{FE0F}]|[\x{E0020}-\x{E007F}]|[\x{1F1E6}-\x{1F1FF}][\x{1F1E6}-\x{1F1FF}]`)

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
	msgID     int
	userID    int64
	firstName string
	username  string
	emoji     []string
}

func getDisplayName(firstName, username string) string {
	if username != "" {
		return "@" + username
	}
	return firstName
}

func requirePrivateChat(ctx context.Context, b *bot.Bot, update *models.Update, i18n func(string, ...map[string]any) string) bool {
	if update.Message.Chat.Type == "private" {
		return true
	}

	botInfo, _ := b.GetMe(ctx)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("sticker-private-only"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{
				Text: i18n("start-button"),
				URL:  fmt.Sprintf("t.me/%s", botInfo.Username),
			}}},
		},
	})
	return false
}

func extractEmojis(text string, replySticker *models.Sticker) []string {
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

func buildSwitchPackList(packs []ValidatedPack, userID int64, i18n func(string, ...map[string]any) string) (string, [][]models.InlineKeyboardButton) {
	var text strings.Builder
	var buttons [][]models.InlineKeyboardButton
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
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         fmt.Sprintf("%d", buttonIndex),
				CallbackData: fmt.Sprintf("switchPack %d %d", userID, pack.ID),
			},
		})
		buttonIndex++
	}

	return text.String(), buttons
}

func editKangError(ctx context.Context, b *bot.Bot, kc *kangContext, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	editErrorMessage(ctx, b, kc.chatID, kc.msgID, i18n, logMsg, err)
}

func editStickerError(ctx context.Context, b *bot.Bot, update *models.Update, progMSG *models.Message, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	editErrorMessage(ctx, b, update.Message.Chat.ID, progMSG.ID, i18n, logMsg, err)
}

func editErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, msgID int, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	slog.Error(logMsg, "Error", err.Error())
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      i18n("kang-error"),
		ParseMode: models.ParseModeHTML,
	})
}

func getStickerHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	if update.Message.ReplyToMessage == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("get-sticker-no-reply-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}
	replySticker := update.Message.ReplyToMessage.Sticker
	if replySticker.IsAnimated {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("get-sticker-animated-not-supported"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if replySticker != nil {
		file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: replySticker.FileID})
		if err != nil {
			slog.Error("Couldn't get file", "Error", err.Error())
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
			return
		}

		response, err := http.Get(b.FileDownloadLink(file))
		if err != nil {
			slog.Error("Couldn't download file", "Error", err.Error())
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
			return
		}
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			slog.Error("Couldn't read body", "Error", err.Error())
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
			return
		}

		filename := filepath.Base(b.FileDownloadLink(file))
		if replySticker.IsVideo {
			filename = fmt.Sprintf("%s.gif", filename[:len(filename)-4])
		}
		_, err = b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID: update.Message.Chat.ID,
			Document: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewBuffer(bodyBytes),
			},
			Caption:                     fmt.Sprintf("<b>Emoji: %s</b>\n<b>ID:</b> <code>%s</code>", replySticker.Emoji, replySticker.FileID),
			ParseMode:                   models.ParseModeHTML,
			DisableContentTypeDetection: *bot.True(),
		})
		if err != nil {
			slog.Error("Couldn't send document", "Error", err.Error())
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: update.Message.ID,
				},
			})
		}
	}
}

func generateStickerSetName(ctx context.Context, b *bot.Bot, userID int64, packNum int) (string, error) {
	botInfo, err := b.GetMe(ctx)
	if err != nil {
		return "", err
	}

	shortNamePrefix := "a_"
	if packNum > 0 {
		shortNamePrefix = fmt.Sprintf("a_%d_", packNum)
	}
	shortNameSuffix := fmt.Sprintf("%d_by_%s", userID, botInfo.Username)

	return shortNamePrefix + shortNameSuffix, nil
}

func syncUserPacks(ctx context.Context, b *bot.Bot, userID int64) error {
	botInfo, err := b.GetMe(ctx)
	if err != nil {
		return err
	}

	suffix := fmt.Sprintf("%d_by_%s", userID, botInfo.Username)

	packNames := make([]string, 0, maxPacks+1)
	packNames = append(packNames, "a_"+suffix) // Legacy format
	for i := range maxPacks {
		packNames = append(packNames, fmt.Sprintf("a_%d_%s", i, suffix))
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, packName := range packNames {
		if exists, _ := packExists(packName); exists {
			continue
		}

		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			if _, err := b.GetStickerSet(ctx, &bot.GetStickerSetParams{Name: name}); err == nil {
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

func getFileIDAndType(reply *models.Message) (stickerAction string, stickerType string, fileID string) {
	if document := reply.Document; document != nil {
		fileID = document.FileID
		switch {
		case strings.Contains(document.MimeType, "image"):
			stickerType = "static"
			stickerAction = "resize"
		case strings.Contains(document.MimeType, "tgsticker"):
			stickerType = "animated"
		case strings.Contains(document.MimeType, "video"):
			stickerType = "video"
			stickerAction = "convert"
		}
	} else {
		switch {
		case reply.Photo != nil:
			stickerType = "static"
			stickerAction = "resize"
			fileID = reply.Photo[len(reply.Photo)-1].FileID
		case reply.Video != nil:
			stickerType = "video"
			stickerAction = "convert"
			fileID = reply.Video.FileID
		case reply.Animation != nil:
			stickerType = "video"
			stickerAction = "convert"
			fileID = reply.Animation.FileID
		}
	}

	if replySticker := reply.Sticker; replySticker != nil {
		if replySticker.IsAnimated {
			stickerType = "animated"
		} else if replySticker.IsVideo {
			stickerType = "video"
		} else {
			stickerType = "static"
		}
		fileID = replySticker.FileID
	}

	return stickerAction, stickerType, fileID
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

	cmd := exec.Command("ffmpeg",
		"-loglevel", "quiet", "-i", inputFile.Name(),
		"-t", "00:00:03", "-vf", "fps=30",
		"-c:v", "vp9", "-b:v", "500k",
		"-preset", "ultrafast", "-s", "512x512",
		"-y", "-an", "-f", "webm",
		tempOutput)

	if err = cmd.Run(); err != nil {
		return nil, err
	}

	outFile, err := os.Open(tempOutput)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	convertedBytes, err := io.ReadAll(outFile)
	if err != nil {
		return nil, err
	}
	return convertedBytes, nil
}

func newPackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	userID := update.Message.From.ID

	if err := syncUserPacks(ctx, b, userID); err != nil {
		slog.Debug("Failed to sync user packs", "Error", err)
	}

	if update.Message.ReplyToMessage == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("kang-no-reply-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	stickerAction, stickerType, fileID := getFileIDAndType(update.Message.ReplyToMessage)
	if stickerType == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-invalid-media-type"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	count, err := getUserPacksCount(userID)
	if err != nil {
		slog.Error("Couldn't get user packs count", "Error", err.Error())
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("kang-error"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if count >= maxPacks {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-max-packs-reached", map[string]any{"maxPacks": maxPacks}),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	progMSG, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("sticker-creating-pack"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
	if err != nil {
		slog.Error("Couldn't send message", "Error", err.Error())
		return
	}

	packName, err := generateStickerSetName(ctx, b, userID, count)
	if err != nil {
		slog.Error("Couldn't generate sticker set name", "Error", err.Error())
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: progMSG.ID,
			Text:      i18n("kang-error"),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	packTitle := generateStickerSetTitle(update.Message.From.FirstName, update.Message.From.Username, count)

	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		editStickerError(ctx, b, update, progMSG, i18n, "Couldn't get file", err)
		return
	}

	response, err := http.Get(b.FileDownloadLink(file))
	if err != nil {
		editStickerError(ctx, b, update, progMSG, i18n, "Couldn't download file", err)
		return
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		editStickerError(ctx, b, update, progMSG, i18n, "Couldn't read body", err)
		return
	}

	switch stickerAction {
	case "resize":
		bodyBytes, err = utils.ResizeSticker(bodyBytes)
		if err != nil {
			slog.Error("Couldn't resize image", "Error", err.Error())
			b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    update.Message.Chat.ID,
				MessageID: progMSG.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
			})
			return
		}
	case "convert":
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: progMSG.ID,
			Text:      i18n("converting-video-to-sticker"),
			ParseMode: models.ParseModeHTML,
		})
		bodyBytes, err = convertVideo(bodyBytes)
		if err != nil {
			slog.Error("Couldn't convert video", "Error", err.Error())
			b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    update.Message.Chat.ID,
				MessageID: progMSG.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
			})
			return
		}
	}

	emoji := extractEmojis(update.Message.Text, update.Message.ReplyToMessage.Sticker)

	stickerFilename := normalizeStickerFilename(filepath.Base(b.FileDownloadLink(file)), stickerAction)

	stickerFile, err := b.UploadStickerFile(ctx, &bot.UploadStickerFileParams{
		UserID: userID,
		Sticker: &models.InputFileUpload{
			Filename: stickerFilename,
			Data:     bytes.NewBuffer(bodyBytes),
		},
		StickerFormat: stickerType,
	})
	if err != nil {
		editStickerError(ctx, b, update, progMSG, i18n, "Couldn't upload sticker file", err)
		return
	}

	_, err = b.CreateNewStickerSet(ctx, &bot.CreateNewStickerSetParams{
		UserID: userID,
		Name:   packName,
		Title:  packTitle,
		Stickers: []models.InputSticker{
			{
				Sticker:   stickerFile.FileID,
				Format:    stickerType,
				EmojiList: emoji,
			},
		},
	})
	if err != nil {
		editStickerError(ctx, b, update, progMSG, i18n, "Couldn't create sticker set", err)
		return
	}

	err = createPack(userID, packName)
	if err != nil {
		slog.Error("Couldn't save pack to database",
			"Error", err.Error())
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: progMSG.ID,
		Text: i18n("sticker-pack-created", map[string]any{
			"packTitle": packTitle,
			"packNum":   count + 1,
		}),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{
				Text: i18n("sticker-newpack-button"),
				URL:  "t.me/addstickers/" + packName,
			}}},
		},
	})
}

func myPacksHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	if !requirePrivateChat(ctx, b, update, i18n) {
		return
	}

	userID := update.Message.From.ID

	if err := syncUserPacks(ctx, b, userID); err != nil {
		slog.Debug("Failed to sync user packs", "Error", err)
	}

	validPacks := validateUserPacks(ctx, b, userID)
	if validPacks == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-no-packs"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	nameTitle := getDisplayName(update.Message.From.FirstName, update.Message.From.Username)

	var text strings.Builder
	text.WriteString(i18n("sticker-mypacks-header", map[string]any{"userName": nameTitle}))
	text.WriteString("\n")

	for i, pack := range validPacks {
		defaultMark := ""
		if pack.IsDefault {
			defaultMark = " ✓"
		}
		fmt.Fprintf(&text, "\n%d — <a href='t.me/addstickers/%s'>%s</a>%s", i+1, pack.PackName, pack.Title, defaultMark)
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	})
}

func validateUserPacks(ctx context.Context, b *bot.Bot, userID int64) []ValidatedPack {
	packs, err := getUserPacks(userID)
	if err != nil {
		return nil
	}

	var validPacks []ValidatedPack
	for _, pack := range packs {
		stickerSet, err := b.GetStickerSet(ctx, &bot.GetStickerSetParams{
			Name: pack.PackName,
		})
		if err != nil {
			slog.Debug("Pack doesn't exist on Telegram, removing from database",
				"PackName", pack.PackName, "UserID", userID)
			deletePack(userID, pack.ID)
			continue
		}
		validPacks = append(validPacks, ValidatedPack{
			StickerPack:  pack,
			Title:        stickerSet.Title,
			StickerCount: len(stickerSet.Stickers),
		})
	}
	return validPacks
}

func switchHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	if !requirePrivateChat(ctx, b, update, i18n) {
		return
	}

	userID := update.Message.From.ID

	if err := syncUserPacks(ctx, b, userID); err != nil {
		slog.Debug("Failed to sync user packs", "Error", err)
	}

	packs := validateUserPacks(ctx, b, userID)
	if packs == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("kang-error"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if len(packs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-no-packs"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if len(packs) == 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-only-one-pack"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	nameTitle := getDisplayName(update.Message.From.FirstName, update.Message.From.Username)

	packListText, buttons := buildSwitchPackList(packs, userID, i18n)

	if len(buttons) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-all-packs-full"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	var text strings.Builder
	text.WriteString(i18n("sticker-switch-header", map[string]any{"userName": nameTitle}))
	text.WriteString(packListText)

	buttons = append(buttons, []models.InlineKeyboardButton{
		{
			Text:         i18n("sticker-switch-none-button"),
			CallbackData: fmt.Sprintf("switchPack %d 0", userID),
		},
	})

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
}

func switchPackCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery.Message.Message == nil {
		return
	}
	i18n := localization.Get(update)

	parts := strings.Split(update.CallbackQuery.Data, " ")
	if len(parts) != 3 {
		return
	}

	ownerID, _ := strconv.ParseInt(parts[1], 10, 64)
	packID, _ := strconv.ParseInt(parts[2], 10, 64)

	if update.CallbackQuery.From.ID != ownerID {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("denied-button-alert"),
			ShowAlert:       true,
		})
		return
	}

	var err error
	if packID == 0 {
		err = clearDefaultPack(ownerID)
	} else {
		err = setDefaultPack(ownerID, packID)
	}

	if err != nil {
		slog.Error("Couldn't update default pack", "Error", err.Error())
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("kang-error"),
			ShowAlert:       true,
		})
		return
	}

	packs := validateUserPacks(ctx, b, ownerID)
	if packs == nil {
		return
	}

	nameTitle := getDisplayName(update.CallbackQuery.From.FirstName, update.CallbackQuery.From.Username)

	packListText, buttons := buildSwitchPackList(packs, ownerID, i18n)

	var text strings.Builder
	text.WriteString(i18n("sticker-switch-header", map[string]any{"userName": nameTitle}))
	text.WriteString(packListText)

	buttons = append(buttons, []models.InlineKeyboardButton{
		{
			Text:         i18n("sticker-switch-none-button"),
			CallbackData: fmt.Sprintf("switchPack %d 0", ownerID),
		},
	})

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        text.String(),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            i18n("sticker-default-changed"),
	})
}

func delPackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	userID := update.Message.From.ID

	if err := syncUserPacks(ctx, b, userID); err != nil {
		slog.Debug("Failed to sync user packs", "Error", err)
	}

	packs := validateUserPacks(ctx, b, userID)
	if packs == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("kang-error"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	if len(packs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("sticker-no-packs"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	nameTitle := getDisplayName(update.Message.From.FirstName, update.Message.From.Username)

	var text strings.Builder
	text.WriteString(i18n("sticker-delpack-header", map[string]any{"userName": nameTitle}))
	text.WriteString("\n")

	var buttons [][]models.InlineKeyboardButton
	for i, pack := range packs {
		fmt.Fprintf(&text, "\n%d — %s", i+1, pack.Title)
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         fmt.Sprintf("%d", i+1),
				CallbackData: fmt.Sprintf("delPack %d %d", userID, pack.ID),
			},
		})
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
}

func delPackCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery.Message.Message == nil {
		return
	}
	i18n := localization.Get(update)

	parts := strings.Split(update.CallbackQuery.Data, " ")
	if len(parts) != 3 {
		return
	}

	ownerID, _ := strconv.ParseInt(parts[1], 10, 64)
	packID, _ := strconv.ParseInt(parts[2], 10, 64)

	if update.CallbackQuery.From.ID != ownerID {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("denied-button-alert"),
			ShowAlert:       true,
		})
		return
	}

	packs, _ := getUserPacks(ownerID)
	var deletedPackName string
	for _, pack := range packs {
		if pack.ID == packID {
			deletedPackName = pack.PackName
			break
		}
	}

	err := deletePack(ownerID, packID)
	if err != nil {
		slog.Error("Couldn't delete pack", "Error", err.Error())
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("kang-error"),
			ShowAlert:       true,
		})
		return
	}

	_, err = b.DeleteStickerSet(ctx, &bot.DeleteStickerSetParams{
		Name: deletedPackName,
	})
	if err != nil {
		slog.Debug("Couldn't delete sticker set from Telegram", "Error", err.Error())
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("sticker-pack-deleted"),
		ParseMode: models.ParseModeHTML,
	})

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}

func kangStickerHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)
	if update.Message.ReplyToMessage == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("kang-no-reply-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}
	progMSG, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("stealing-sticker"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
	if err != nil {
		slog.Error("Couldn't send message", "Error", err.Error())
		return
	}

	stickerAction, stickerType, fileID := getFileIDAndType(update.Message.ReplyToMessage)
	if stickerType == "" {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: progMSG.ID,
			Text:      i18n("sticker-invalid-media-type"),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	userID := update.Message.From.ID

	if err := syncUserPacks(ctx, b, userID); err != nil {
		slog.Debug("Failed to sync user packs", "Error", err)
	}

	packs, err := getUserPacks(userID)
	if err != nil {
		slog.Error("Couldn't get user packs", "Error", err.Error())
	}

	if len(packs) == 0 {
		packName, err := generateStickerSetName(ctx, b, userID, 0)
		if err != nil {
			editStickerError(ctx, b, update, progMSG, i18n, "Couldn't generate sticker set name", err)
			return
		}

		err = createPack(userID, packName)
		if err != nil {
			editStickerError(ctx, b, update, progMSG, i18n, "Couldn't create pack", err)
			return
		}

		packs, _ = getUserPacks(userID)
	}

	defaultPack, _ := getDefaultPack(userID)
	if len(packs) > 1 && defaultPack == nil {
		showPackSelection(ctx, b, update, progMSG, packs, i18n, stickerAction, stickerType, fileID)
		return
	}

	var targetPack *StickerPack
	if defaultPack != nil {
		targetPack = defaultPack
	} else if len(packs) > 0 {
		targetPack = &packs[0]
	}

	if targetPack == nil {
		editStickerError(ctx, b, update, progMSG, i18n, "No pack available", fmt.Errorf("no pack found"))
		return
	}

	processKang(ctx, b, update, progMSG, i18n, targetPack, stickerAction, stickerType, fileID)
}

func showPackSelection(ctx context.Context, b *bot.Bot, update *models.Update, progMSG *models.Message, packs []StickerPack, i18n func(string, ...map[string]any) string, stickerAction, stickerType, fileID string) {
	nameTitle := getDisplayName(update.Message.From.FirstName, update.Message.From.Username)

	var text strings.Builder
	text.WriteString(i18n("sticker-select-pack", map[string]any{"userName": nameTitle}))
	text.WriteString("\n")

	var replySticker *models.Sticker
	if update.Message.ReplyToMessage != nil {
		replySticker = update.Message.ReplyToMessage.Sticker
	}
	emojiStr := extractEmojis(update.Message.Text, replySticker)[0]

	kangData := KangData{
		StickerAction: stickerAction,
		StickerType:   stickerType,
		FileID:        fileID,
		Emoji:         emojiStr,
	}
	kangDataJSON, _ := json.Marshal(kangData)
	cacheKey := fmt.Sprintf("kang:%d:%d", update.Message.From.ID, progMSG.ID)
	cache.SetCache(cacheKey, string(kangDataJSON), 5*time.Minute)

	var buttons [][]models.InlineKeyboardButton
	for i, pack := range packs {
		stickerSet, err := b.GetStickerSet(ctx, &bot.GetStickerSetParams{Name: pack.PackName})
		if err != nil {
			continue
		}
		if len(stickerSet.Stickers) >= 120 {
			fmt.Fprintf(&text, "\nX — <a href='t.me/addstickers/%s'>%s</a> <b>%s</b>", pack.PackName, stickerSet.Title, i18n("sticker-pack-full-mark"))
			continue
		}
		fmt.Fprintf(&text, "\n%d — <a href='t.me/addstickers/%s'>%s</a>", i+1, pack.PackName, stickerSet.Title)
		callbackData := fmt.Sprintf("kangPack %d %d", progMSG.ID, pack.ID)
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         fmt.Sprintf("%d", i+1),
				CallbackData: callbackData,
			},
		})
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: progMSG.ID,
		Text:      text.String(),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
}

func kangPackCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery.Message.Message == nil {
		return
	}
	i18n := localization.Get(update)

	parts := strings.Split(update.CallbackQuery.Data, " ")
	if len(parts) != 3 {
		return
	}

	msgID, _ := strconv.Atoi(parts[1])
	packID, _ := strconv.ParseInt(parts[2], 10, 64)
	userID := update.CallbackQuery.From.ID

	cacheKey := fmt.Sprintf("kang:%d:%d", userID, msgID)
	kangDataJSON, err := cache.GetCache(cacheKey)
	if err != nil || kangDataJSON == "" {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("sticker-kang-expired"),
			ShowAlert:       true,
		})
		return
	}

	var kangData KangData
	if err := json.Unmarshal([]byte(kangDataJSON), &kangData); err != nil {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("kang-error"),
			ShowAlert:       true,
		})
		return
	}

	packs, _ := getUserPacks(userID)
	var targetPack *StickerPack
	for _, pack := range packs {
		if pack.ID == packID {
			targetPack = &pack
			break
		}
	}

	if targetPack == nil {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("kang-error"),
			ShowAlert:       true,
		})
		return
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("stealing-sticker"),
		ParseMode: models.ParseModeHTML,
	})

	processKangFromCallback(ctx, b, update, i18n, targetPack, kangData.StickerAction, kangData.StickerType, kangData.FileID, kangData.Emoji)

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}

func processKangFromCallback(ctx context.Context, b *bot.Bot, update *models.Update, i18n func(string, ...map[string]any) string, pack *StickerPack, stickerAction, stickerType, fileID, emojiStr string) {
	kc := &kangContext{
		chatID:    update.CallbackQuery.Message.Message.Chat.ID,
		msgID:     update.CallbackQuery.Message.Message.ID,
		userID:    update.CallbackQuery.From.ID,
		firstName: update.CallbackQuery.From.FirstName,
		username:  update.CallbackQuery.From.Username,
		emoji:     []string{emojiStr},
	}
	processKangSticker(ctx, b, i18n, kc, pack, stickerAction, stickerType, fileID)
}

func processKang(ctx context.Context, b *bot.Bot, update *models.Update, progMSG *models.Message, i18n func(string, ...map[string]any) string, pack *StickerPack, stickerAction, stickerType, fileID string) {
	var replySticker *models.Sticker
	if update.Message.ReplyToMessage != nil {
		replySticker = update.Message.ReplyToMessage.Sticker
	}

	kc := &kangContext{
		chatID:    update.Message.Chat.ID,
		msgID:     progMSG.ID,
		userID:    update.Message.From.ID,
		firstName: update.Message.From.FirstName,
		username:  update.Message.From.Username,
		emoji:     extractEmojis(update.Message.Text, replySticker),
	}
	processKangSticker(ctx, b, i18n, kc, pack, stickerAction, stickerType, fileID)
}

func processKangSticker(ctx context.Context, b *bot.Bot, i18n func(string, ...map[string]any) string, kc *kangContext, pack *StickerPack, stickerAction, stickerType, fileID string) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		editKangError(ctx, b, kc, i18n, "Couldn't get file", err)
		return
	}

	response, err := http.Get(b.FileDownloadLink(file))
	if err != nil {
		editKangError(ctx, b, kc, i18n, "Couldn't download file", err)
		return
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		editKangError(ctx, b, kc, i18n, "Couldn't read body", err)
		return
	}

	switch stickerAction {
	case "resize":
		bodyBytes, err = utils.ResizeSticker(bodyBytes)
		if err != nil {
			editKangError(ctx, b, kc, i18n, "Couldn't resize image", err)
			return
		}
	case "convert":
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    kc.chatID,
			MessageID: kc.msgID,
			Text:      i18n("converting-video-to-sticker"),
			ParseMode: models.ParseModeHTML,
		})
		bodyBytes, err = convertVideo(bodyBytes)
		if err != nil {
			editKangError(ctx, b, kc, i18n, "Couldn't convert video", err)
			return
		}
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    kc.chatID,
		MessageID: kc.msgID,
		Text:      i18n("sticker-pack-already-exists"),
		ParseMode: models.ParseModeHTML,
	})

	stickerFilename := normalizeStickerFilename(filepath.Base(b.FileDownloadLink(file)), stickerAction)

	stickerFile, err := b.UploadStickerFile(ctx, &bot.UploadStickerFileParams{
		UserID: kc.userID,
		Sticker: &models.InputFileUpload{
			Filename: stickerFilename,
			Data:     bytes.NewBuffer(bodyBytes),
		},
		StickerFormat: stickerType,
	})
	if err != nil {
		editKangError(ctx, b, kc, i18n, "Couldn't upload sticker file", err)
		return
	}

	stickerSet, err := b.GetStickerSet(ctx, &bot.GetStickerSetParams{Name: pack.PackName})
	if err == nil && len(stickerSet.Stickers) >= 120 {
		kangData := KangData{
			StickerAction: stickerAction,
			StickerType:   stickerType,
			FileID:        stickerFile.FileID,
			Emoji:         strings.Join(kc.emoji, ""),
		}
		kangDataJSON, _ := json.Marshal(kangData)
		cacheKey := fmt.Sprintf("kangFull:%d:%d", kc.userID, kc.msgID)
		cache.SetCache(cacheKey, string(kangDataJSON), 5*time.Minute)

		if packDefault, err := getDefaultPack(kc.userID); err == nil &&
			packDefault != nil && packDefault.PackName == pack.PackName {
			if err := clearDefaultPack(kc.userID); err != nil {
				slog.Error("Failed to clear default pack",
					"error", err.Error())
			}
		}

		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    kc.chatID,
			MessageID: kc.msgID,
			Text:      i18n("sticker-pack-full", map[string]any{"packName": stickerSet.Title, "stickerCount": len(stickerSet.Stickers)}),
			ParseMode: models.ParseModeHTML,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{{
					Text:         i18n("sticker-create-new-pack-button"),
					CallbackData: fmt.Sprintf("createNewPack %d", kc.msgID),
				}}},
			},
		})
		return
	}

	_, err = b.AddStickerToSet(ctx, &bot.AddStickerToSetParams{
		UserID: kc.userID,
		Name:   pack.PackName,
		Sticker: models.InputSticker{
			Sticker:   stickerFile.FileID,
			Format:    stickerType,
			EmojiList: kc.emoji,
		},
	})
	if err != nil {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    kc.chatID,
			MessageID: kc.msgID,
			Text:      i18n("sticker-new-pack"),
			ParseMode: models.ParseModeHTML,
		})

		_, err = b.CreateNewStickerSet(ctx, &bot.CreateNewStickerSetParams{
			UserID: kc.userID,
			Name:   pack.PackName,
			Title:  generateStickerSetTitle(kc.firstName, kc.username, 0),
			Stickers: []models.InputSticker{
				{
					Sticker:   stickerFile.FileID,
					Format:    stickerType,
					EmojiList: kc.emoji,
				},
			},
		})
		if err != nil {
			editKangError(ctx, b, kc, i18n, "Couldn't create sticker set", err)
			return
		}
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    kc.chatID,
		MessageID: kc.msgID,
		Text: i18n("sticker-stoled", map[string]any{
			"emoji": strings.Join(kc.emoji, ""),
		}),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{
				Text: i18n("sticker-stoled-button"),
				URL:  "t.me/addstickers/" + pack.PackName,
			}}},
		},
	})
}

func createNewPackCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery.Message.Message == nil {
		return
	}
	i18n := localization.Get(update)
	userID := update.CallbackQuery.From.ID

	parts := strings.Split(update.CallbackQuery.Data, " ")
	if len(parts) != 2 {
		return
	}

	msgID, _ := strconv.Atoi(parts[1])

	cacheKey := fmt.Sprintf("kangFull:%d:%d", userID, msgID)
	kangDataJSON, err := cache.GetCache(cacheKey)
	if err != nil || kangDataJSON == "" {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("sticker-kang-expired"),
			ShowAlert:       true,
		})
		return
	}

	var kangData KangData
	if err := json.Unmarshal([]byte(kangDataJSON), &kangData); err != nil {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("kang-error"),
			ShowAlert:       true,
		})
		return
	}

	count, err := getUserPacksCount(userID)
	if err != nil {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            i18n("kang-error"),
			ShowAlert:       true,
		})
		return
	}

	if count >= maxPacks {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      i18n("sticker-max-packs-reached", map[string]any{"maxPacks": maxPacks}),
			ParseMode: models.ParseModeHTML,
		})
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
		return
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      i18n("sticker-creating-pack"),
		ParseMode: models.ParseModeHTML,
	})

	packName, err := generateStickerSetName(ctx, b, userID, count)
	if err != nil {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      i18n("kang-error"),
			ParseMode: models.ParseModeHTML,
		})
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
		return
	}

	packTitle := generateStickerSetTitle(update.CallbackQuery.From.FirstName, update.CallbackQuery.From.Username, count)

	emoji := []string{kangData.Emoji}
	if kangData.Emoji == "" {
		emoji = []string{"🤔"}
	}

	_, err = b.CreateNewStickerSet(ctx, &bot.CreateNewStickerSetParams{
		UserID: userID,
		Name:   packName,
		Title:  packTitle,
		Stickers: []models.InputSticker{
			{
				Sticker:   kangData.FileID,
				Format:    kangData.StickerType,
				EmojiList: emoji,
			},
		},
	})
	if err != nil {
		slog.Error("Couldn't create sticker set", "Error", err.Error())
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      i18n("kang-error"),
			ParseMode: models.ParseModeHTML,
		})
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
		return
	}

	err = createPack(userID, packName)
	if err != nil {
		slog.Error("Couldn't save pack to database", "Error", err.Error())
	}

	cache.DeleteCache(cacheKey)

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text: i18n("sticker-pack-created", map[string]any{
			"packTitle": packTitle,
			"packNum":   count + 1,
		}),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{
				Text: i18n("sticker-newpack-button"),
				URL:  "t.me/addstickers/" + packName,
			}}},
		},
	})

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeCommand, "getsticker", getStickerHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "kang", kangStickerHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "newpack", newPackHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "mypacks", myPacksHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "switch", switchHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "delpack", delPackHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "^switchPack", switchPackCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "^delPack", delPackCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "^kangPack", kangPackCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "^createNewPack", createNewPackCallback)

	utils.SaveHelp("stickers")
	utils.DisableableCommands = append(utils.DisableableCommands, "getsticker")
}
