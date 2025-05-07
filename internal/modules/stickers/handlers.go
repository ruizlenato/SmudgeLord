package stickers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	stickerTypeStatic   = "static"
	stickerTypeAnimated = "animated"
	stickerTypeVideo    = "video"
	actionResize        = "resize"
	actionConvert       = "convert"
)

var emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{2694}-\x{2697}]|[\x{2702}-\x{27B0}]|[\x{1F926}-\x{1F937}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{2600}-\x{26FF}]`)

func getStickerHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)

	if !message.IsReply() {
		_, err := message.Reply(i18n("get-sticker-no-reply-provided"))
		return err
	}

	reply, err := message.GetReplyMessage()
	if err != nil {
		slog.Error(
			"Failed to get reply message",
			"error", err.Error(),
		)
		return err
	}

	if reply.Sticker() == nil {
		_, err := message.Reply(i18n("get-sticker-no-reply-provided"))
		return err
	}

	stickerType, emoji := extractStickerInfo(reply)

	if stickerType == stickerTypeAnimated {
		_, err := message.Reply(i18n("get-sticker-animated-not-supported"))
		return err
	}

	var buf bytes.Buffer
	_, err = reply.Download(&telegram.DownloadOptions{Buffer: &buf})
	if err != nil {
		slog.Error(
			"Failed to download sticker",
			"error", err.Error(),
		)
		return err
	}

	filename := "sticker.png"
	if stickerType == "video" {
		filename = "sticker.webm"
	}

	_, err = message.ReplyMedia(buf.Bytes(), telegram.MediaOptions{
		ForceDocument: true,
		Caption:       "<b>Emoji:</b> " + emoji,
		FileName:      filename,
	})
	if err != nil {
		slog.Error(
			"Error sending sticker",
			"error", err.Error(),
		)
	}
	return err
}

func kangStickerHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	if !message.IsReply() {
		_, err := message.Reply(i18n("kang-no-reply-provided"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	progressMessage, err := message.Reply(i18n("stealing-sticker"), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
	if err != nil {
		return err
	}

	reply, err := message.GetReplyMessage()
	if err != nil || !reply.IsMedia() && reply.Sticker() == nil {
		return nil
	}

	stickerType, stickerEmoji := extractStickerInfo(reply)
	if stickerEmoji == "" {
		stickerEmoji = "🤔"
	}

	if emoji := emojiRegex.FindString(reply.Text()); emoji != "" {
		stickerEmoji = emoji
	}

	var action string
	if reply.IsMedia() && reply.Sticker() == nil {
		if _, ok := reply.Media().(*telegram.MessageMediaPhoto); ok {
			action = actionResize
		}
		if media, ok := reply.Media().(*telegram.MessageMediaDocument); ok {
			if document, ok := media.Document.(*telegram.DocumentObj); ok {
				switch {
				case strings.Contains(document.MimeType, "image"):
					action = actionResize
				case strings.Contains(document.MimeType, "video"):
					action = actionConvert
				}
			}
		}
	}

	var buf bytes.Buffer
	_, err = reply.Download(&telegram.DownloadOptions{Buffer: &buf})
	if err != nil {
		slog.Error(
			"Failed to download file",
			"error", err.Error(),
		)
		return err
	}
	stickerBytes := buf.Bytes()

	var filename string

	switch action {
	case actionResize:
		stickerBytes, err = utils.ResizeSticker(buf.Bytes())
		if err != nil {
			return editProgressAndReturn(progressMessage, i18n("kang-error"), err)
		}
		filename = "sticker.png"
	case actionConvert:
		progressMessage, err = progressMessage.Edit(i18n("converting-video-to-sticker"), telegram.SendOptions{ParseMode: telegram.HTML})
		if err != nil {
			return err
		}
		stickerBytes, err = convertVideoBytes(stickerBytes)
		if err != nil {
			return editProgressAndReturn(progressMessage, i18n("kang-error"), err)
		}
		filename = "sticker.webm"
	default:
		switch stickerType {
		case stickerTypeStatic:
			filename = "sticker.png"
		case stickerTypeVideo:
			filename = "sticker.webm"
		case stickerTypeAnimated:
			filename = "sticker.tgs"
		default:
			return editProgressAndReturn(progressMessage, i18n("sticker-invalid-media-type"), nil)
		}
	}

	stickerSetShortName, stickerSetTitle := generateStickerSetName(message)
	mediaMsg, err := message.Client.SendMedia(config.ChannelLogID, stickerBytes, &telegram.MediaOptions{
		ForceDocument: true,
		FileName:      filename,
	})
	if err != nil {
		return editProgressAndReturn(progressMessage, i18n("kang-error"), err)
	}
	defer func() {
		if _, err := mediaMsg.Delete(); err != nil {
			slog.Error(
				"Could not delete media message",
				"mediaMessageID", mediaMsg.ID,
				"error", err.Error(),
			)
		}
	}()

	progressMessage, err = progressMessage.Edit(i18n("sticker-pack-already-exists"), telegram.SendOptions{ParseMode: telegram.HTML})
	if err != nil {
		return err
	}

	_, err = message.Client.StickersAddStickerToSet(&telegram.InputStickerSetShortName{ShortName: stickerSetShortName}, &telegram.InputStickerSetItem{
		Document: &telegram.InputDocumentObj{ID: mediaMsg.Document().ID, AccessHash: mediaMsg.Document().AccessHash},
		Emoji:    stickerEmoji,
	})
	if err != nil {
		progressMessage, err = progressMessage.Edit(i18n("sticker-new-pack"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		if err != nil {
			return err
		}
		_, err = message.Client.StickersCreateStickerSet(&telegram.StickersCreateStickerSetParams{
			UserID:    &telegram.InputUserObj{UserID: message.Sender.ID, AccessHash: message.Sender.AccessHash},
			ShortName: stickerSetShortName,
			Title:     stickerSetTitle,
			Stickers: []*telegram.InputStickerSetItem{{
				Document: &telegram.InputDocumentObj{ID: mediaMsg.Document().ID, AccessHash: mediaMsg.Document().AccessHash},
				Emoji:    stickerEmoji,
			}},
		})
		if err != nil {
			return editProgressAndReturn(progressMessage, i18n("kang-error"), err)
		}
	}
	_, err = progressMessage.Edit(i18n("sticker-stoled",
		map[string]any{
			"stickerSetName": stickerSetShortName,
			"emoji":          stickerEmoji,
		}), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
	return err
}

func editProgressAndReturn(progressMessage *telegram.NewMessage, text string, originalErr error) error {
	_, editErr := progressMessage.Edit(text, telegram.SendOptions{ParseMode: telegram.HTML})
	if editErr != nil {
		slog.Error("Failed to edit progress message on error", "editError", editErr, "originalError", originalErr)
	}
	return originalErr
}

func generateStickerSetName(message *telegram.NewMessage) (string, string) {
	shortNamePrefix := "a_"
	shortNameSuffix := fmt.Sprintf("%d_by_%s", message.SenderID(), message.Client.Me().Username)
	nameTitle := message.Sender.FirstName
	if username := message.Sender.Username; username != "" {
		nameTitle = "@" + username
	}
	if len(nameTitle) > 35 {
		nameTitle = nameTitle[:35]
	}
	stickerSetTitle := fmt.Sprintf("%s's SmudgeLord", nameTitle)
	stickerSetShortName := shortNamePrefix + shortNameSuffix

	for i := 0; checkStickerSetCount(message, stickerSetShortName); i++ {
		stickerSetShortName = fmt.Sprintf("%s%d_%s", shortNamePrefix, i, shortNameSuffix)
	}
	return stickerSetShortName, stickerSetTitle
}

func checkStickerSetCount(message *telegram.NewMessage, stickerSetName string) bool {
	stickerSet, err := message.Client.MessagesGetStickerSet(&telegram.InputStickerSetShortName{ShortName: stickerSetName}, 0)
	if err != nil {
		return false
	}
	if stickerSetObj, ok := stickerSet.(*telegram.MessagesStickerSetObj); ok {
		return stickerSetObj.Set.Count > 120
	}
	return false
}

func extractStickerInfo(reply *telegram.NewMessage) (stickerType string, emoji string) {
	if reply.Sticker() != nil {
		if media, ok := reply.Media().(*telegram.MessageMediaDocument); ok {
			if document, ok := media.Document.(*telegram.DocumentObj); ok {
				switch {
				case strings.Contains(document.MimeType, "image"):
					stickerType = stickerTypeStatic
				case strings.Contains(document.MimeType, "tgsticker"):
					stickerType = stickerTypeAnimated
				case strings.Contains(document.MimeType, "video"):
					stickerType = stickerTypeVideo
				}
				for _, attr := range document.Attributes {
					if attrSticker, ok := attr.(*telegram.DocumentAttributeSticker); ok {
						emoji = attrSticker.Alt
						break
					}
				}
			}
		}
		return stickerType, emoji
	}

	return stickerType, emoji
}

func convertVideoBytes(input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inputFile, err := os.CreateTemp("", "Smudgeinput_*.mp4")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inputFile.Name())

	_, err = inputFile.Write(input)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("ffmpeg",
		"-loglevel", "quiet",
		"-i", inputFile.Name(),
		"-t", "00:00:03",
		"-vf", "fps=30",
		"-s", "512x512",
		"-c:v", "libvpx-vp9",
		"-b:v", "256k",
		"-preset", "ultrafast",
		"-y", "-an",
		"-f", "webm",
		"pipe:1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("ffmpeg conversion timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("ffmpeg error: %w\nStderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

func Load(client *telegram.Client) {
	utils.SotreHelp("stickers")
	client.On("command:getsticker", handlers.HandleCommand(getStickerHandler))
	client.On("command:kang", handlers.HandleCommand(kangStickerHandler))

	handlers.DisableableCommands = append(handlers.DisableableCommands, "getsticker", "kang")
}
