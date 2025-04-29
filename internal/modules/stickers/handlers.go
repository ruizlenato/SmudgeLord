package stickers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

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

	if replySticker := update.Message.ReplyToMessage.Sticker; replySticker != nil && !replySticker.IsAnimated {
		file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: replySticker.FileID})
		if err != nil {
			slog.Error("Couldn't get file",
				"Error", err.Error())
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
			slog.Error("Couldn't download file",
				"Error", err.Error())
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
			slog.Error("Couldn't read body",
				"Error", err.Error())
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

		_, err = b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID: update.Message.Chat.ID,
			Document: &models.InputFileUpload{
				Filename: filepath.Base(b.FileDownloadLink(file)),
				Data:     bytes.NewBuffer(bodyBytes),
			},
			Caption:                     fmt.Sprintf("<b>Emoji: %s</b>\n<b>ID:</b> <code>%s</code>", replySticker.Emoji, replySticker.FileID),
			ParseMode:                   models.ParseModeHTML,
			DisableContentTypeDetection: *bot.True(),
		})
		if err != nil {
			slog.Error("Couldn't send document",
				"Error", err.Error())
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

func editStickerError(ctx context.Context, b *bot.Bot, update *models.Update, progMSG *models.Message, i18n func(string, ...map[string]any) string, logMsg string, err error) {
	slog.Error(logMsg,
		"Error", err.Error())
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: progMSG.ID,
		Text:      i18n("kang-error"),
		ParseMode: models.ParseModeHTML,
	})
}

func generateStickerSetName(ctx context.Context, b *bot.Bot, update *models.Update) (string, string) {
	botInfo, err := b.GetMe(ctx)
	if err != nil {
		slog.Error("Couldn't get bot info",
			"Error", err.Error())
		os.Exit(1)

	}

	shortNamePrefix := "a_"
	shortNameSuffix := fmt.Sprintf("%d_by_%s", update.Message.From.ID, botInfo.Username)

	nameTitle := update.Message.From.FirstName
	if username := update.Message.From.Username; username != "" {
		nameTitle = "@" + username
	}
	if len(nameTitle) > 35 {
		nameTitle = nameTitle[:35]
	}
	stickerSetTitle := fmt.Sprintf("%s's SmudgeLord", nameTitle)
	stickerSetShortName := shortNamePrefix + shortNameSuffix

	for i := 0; checkStickerSetCount(ctx, b, stickerSetShortName); i++ {
		stickerSetShortName = fmt.Sprintf("%s%d_%s", shortNamePrefix, i, shortNameSuffix)
	}
	return stickerSetShortName, stickerSetTitle
}

func checkStickerSetCount(ctx context.Context, b *bot.Bot, stickerSetShortName string) bool {
	stickerSet, err := b.GetStickerSet(ctx, &bot.GetStickerSetParams{
		Name: stickerSetShortName,
	})
	if err != nil {
		return false
	}
	if len(stickerSet.Stickers) > 120 {
		return true
	}
	return false
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
		Text:      i18n("kanging"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
	if err != nil {
		slog.Error("Couldn't send message",
			"Error", err.Error())
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

	var (
		emoji []string
	)

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
			slog.Error("Couldn't resize image",
				"Error", err.Error())
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
			slog.Error("Couldn't convert video",
				"Error", err.Error())
			b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    update.Message.Chat.ID,
				MessageID: progMSG.ID,
				Text:      i18n("kang-error"),
				ParseMode: models.ParseModeHTML,
			})
			return
		}
	}

	stickerSetShortName, stickerSetTitle := generateStickerSetName(ctx, b, update)
	reEmoji := regexp.MustCompile(`[\x{1F000}-\x{1FAFF}]|[\x{2600}-\x{27BF}]|\x{200D}|[\x{FE00}-\x{FE0F}]|[\x{E0020}-\x{E007F}]|[\x{1F1E6}-\x{1F1FF}][\x{1F1E6}-\x{1F1FF}]`)
	emoji = append(emoji, reEmoji.FindAllString(update.Message.Text, -1)...)

	if len(emoji) == 0 && update.Message.ReplyToMessage.Sticker != nil {
		emoji = append(emoji, update.Message.ReplyToMessage.Sticker.Emoji)
	}

	if len(emoji) == 0 {
		emoji = append(emoji, "🤔")
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: progMSG.ID,
		Text:      i18n("sticker-pack-already-exists"),
		ParseMode: models.ParseModeHTML,
	})

	stickerFilename := filepath.Base(b.FileDownloadLink(file))
	if stickerAction == "resize" && filepath.Ext(stickerFilename) != ".webp" || stickerAction == "resize" && filepath.Ext(stickerFilename) != ".png" {
		stickerFilename = strings.TrimSuffix(stickerFilename, filepath.Ext(stickerFilename)) + ".png"
	}

	stickerFile, err := b.UploadStickerFile(ctx, &bot.UploadStickerFileParams{
		UserID: update.Message.From.ID,
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

	_, err = b.AddStickerToSet(ctx, &bot.AddStickerToSetParams{
		UserID: update.Message.From.ID,
		Name:   stickerSetShortName,
		Sticker: models.InputSticker{
			Sticker: &models.InputFileString{
				Data: stickerFile.FileID,
			},
			Format:    stickerType,
			EmojiList: emoji,
		},
	})
	if err != nil {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: progMSG.ID,
			Text:      i18n("sticker-new-pack"),
			ParseMode: models.ParseModeHTML,
		})

		_, err = b.CreateNewStickerSet(ctx, &bot.CreateNewStickerSetParams{
			UserID: update.Message.From.ID,
			Name:   stickerSetShortName,
			Title:  stickerSetTitle,
			Stickers: []models.InputSticker{
				{
					Sticker: &models.InputFileString{
						Data: stickerFile.FileID,
					},
					Format:    stickerType,
					EmojiList: emoji,
				},
			},
		})
		if err != nil {
			editStickerError(ctx, b, update, progMSG, i18n, "Couldn't add sticker to set", err)
			return
		}
	}

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: progMSG.ID,
		Text: i18n("sticker-stoled",
			map[string]any{
				"stickerSetName": stickerSetShortName,
				"emoji":          strings.Join(emoji, ""),
			}),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	})
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "getsticker", bot.MatchTypeCommand, getStickerHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "kang", bot.MatchTypeCommand, kangStickerHandler)

	utils.SaveHelp("stickers")
	utils.DisableableCommands = append(utils.DisableableCommands,
		"getsticker")
}
