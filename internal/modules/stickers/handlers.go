package stickers

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{2694}-\x{2697}]|[\x{2702}-\x{27B0}]|[\x{1F926}-\x{1F937}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{2600}-\x{26FF}]`)

func handlerGetSticker(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	if !message.IsReply() {
		_, err := message.Reply(i18n("get-sticker-no-reply-provided"))
		return err
	}

	reply, err := message.GetReplyMessage()
	if err != nil {
		return err
	}

	stickerType, emoji := extractStickerInfo(reply)
	if reply.Sticker() == nil || stickerType == "animated" {
		_, err := message.Reply(i18n("get-sticker-no-reply-provided"))
		return err
	}

	filename := "Smudge*.png"
	if stickerType == "video" {
		filename = "Smudge*.gif"
	}
	file, err := os.CreateTemp("", filename)
	if err != nil {
		return err
	}

	stickerFile, err := reply.Download(&telegram.DownloadOptions{FileName: file.Name()})
	if err != nil {
		return err
	}

	_, err = message.ReplyMedia(stickerFile, telegram.MediaOptions{ForceDocument: true, Caption: "<b>Emoji:</b> " + emoji})
	return err
}

func handlerKangSticker(message *telegram.NewMessage) error {
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
	if err != nil {
		return err
	}

	if !reply.IsMedia() && reply.Sticker() == nil {
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
			action = "resize"
		}
		if media, ok := reply.Media().(*telegram.MessageMediaDocument); ok {
			if document, ok := media.Document.(*telegram.DocumentObj); ok {
				switch {
				case strings.Contains(document.MimeType, "image"):
					action = "resize"
				case strings.Contains(document.MimeType, "video"):
					action = "convert"
				}
			}
		}
	}

	var file *os.File
	switch {
	case action == "resize", stickerType == "static":
		file, err = os.CreateTemp("", "*"+".png")
	case action == "convert", stickerType == "video":
		file, err = os.CreateTemp("", "*"+".webm")
	case stickerType == "animated":
		file, err = os.CreateTemp("", "*"+".tgs")
	default:
		_, err = progressMessage.Edit(i18n("sticker-invalid-media-type"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}
	if err != nil {
		return err
	}

	defer func() {
		if err := os.Remove(file.Name()); err != nil {
			slog.Error("Could not remove file",
				"file", file.Name(),
				"error", err.Error())
			return
		}
	}()

	stickerFile, err := reply.Download(&telegram.DownloadOptions{FileName: file.Name()})
	if err != nil {
		return err
	}

	switch action {
	case "resize":
		err = resizeImage(stickerFile)
		if err != nil {
			_, err := progressMessage.Edit(i18n("kang-error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			if err != nil {
				return err
			}
			return err
		}
	case "convert":
		progressMessage, err = progressMessage.Edit(i18n("converting-video-to-sticker"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		if err != nil {
			return err
		}
		err = convertVideo(stickerFile)
		if err != nil {
			_, err := progressMessage.Edit(i18n("kang-error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			if err != nil {
				return err
			}
			return err
		}
	}

	stickerSetShortName, stickerSetTitle := generateStickerSetName(message)
	mediaMsg, err := message.Client.SendMedia(config.ChannelLogID, stickerFile, &telegram.MediaOptions{
		ForceDocument: true,
	})
	if err != nil {
		_, err = progressMessage.Edit(i18n("kang-error"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}
	defer func() {
		if _, err := mediaMsg.Delete(); err != nil {
			slog.Error("Could not delete media message",
				"mediaMessageID", mediaMsg.ID,
				"error", err.Error())
		}
	}()

	progressMessage, err = progressMessage.Edit(i18n("sticker-pack-already-exists"), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
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
			_, err = progressMessage.Edit(i18n("kang-error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
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
					stickerType = "static"
				case strings.Contains(document.MimeType, "tgsticker"):
					stickerType = "animated"
				case strings.Contains(document.MimeType, "video"):
					stickerType = "video"
				}
				for _, attr := range document.Attributes {
					if attrSticker, ok := attr.(*telegram.DocumentAttributeSticker); ok {
						emoji = attrSticker.Alt
					}
				}
			}
		}
		return stickerType, emoji
	}

	return stickerType, emoji
}

func resizeImage(input string) error {
	img, err := imgio.Open(input)
	if err != nil {
		return err
	}

	resizedImg := transform.Resize(img, 512, 512, transform.Lanczos)
	return imgio.Save(input, resizedImg, imgio.PNGEncoder())
}

func convertVideo(inputFile string) (err error) {
	outputFile, err := os.CreateTemp("", "Smudge*.webm")
	if err != nil {
		return err
	}
	defer os.Remove(outputFile.Name())

	err = exec.Command("ffmpeg",
		"-i", inputFile,
		"-t", "00:00:03",
		"-vf", "fps=30,scale=512:512",
		"-c:v", "libvpx-vp9",
		"-b:v", "256k",
		"-preset", "ultrafast",
		"-an",
		"-y",
		outputFile.Name(),
	).Run()
	if err != nil {
		return err
	}
	outputFile.Seek(0, 0)

	return os.Rename(outputFile.Name(), inputFile)
}

func Load(client *telegram.Client) {
	utils.SotreHelp("stickers")
	client.On("command:getsticker", handlers.HandleCommand(handlerGetSticker))
	client.On("command:kang", handlers.HandleCommand(handlerKangSticker))

	handlers.DisableableCommands = append(handlers.DisableableCommands, "getsticker", "kang")
}
