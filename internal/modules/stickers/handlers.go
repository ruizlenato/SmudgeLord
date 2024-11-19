package stickers

import (
	"fmt"
	"image"
	"log"
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
		_, err := message.Reply(i18n("stickers.getNotReply"))
		return err
	}

	reply, err := message.GetReplyMessage()
	if err != nil {
		return err
	}

	stickerType, emoji := extractStickerInfo(reply)
	if reply.Sticker() == nil || stickerType == "animated" {
		_, err := message.Reply(i18n("stickers.getNotReply"))
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
		_, err := message.Reply(i18n("stickers.kangNotReply"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	progressMessage, err := message.Reply(i18n("stickers.kanging"), telegram.SendOptions{
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
		return nil
	}
	if err != nil {
		return err
	}

	defer func() {
		if err := os.Remove(file.Name()); err != nil {
			log.Print(err)
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
			_, err := progressMessage.Edit(i18n("stickers.error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			if err != nil {
				return err
			}
			return err
		}
	case "convert":
		fmt.Println("Converting video")
	}

	stickerSetShortName, stickerSetTitle := generateStickerSetName(message)
	mediaMsg, err := message.Client.SendMedia(config.ChannelLogID, stickerFile, &telegram.MediaOptions{
		ForceDocument: true,
	})
	if err != nil {
		progressMessage, err = progressMessage.Edit(i18n("stickers.error"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}
	defer func() {
		if _, err := mediaMsg.Delete(); err != nil {
			log.Println(err)
		}
	}()

	progressMessage, err = progressMessage.Edit(i18n("stickers.packExists"), telegram.SendOptions{
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
		progressMessage, err = progressMessage.Edit(i18n("stickers.newPack"), telegram.SendOptions{
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
			progressMessage, err = progressMessage.Edit(i18n("stickers.error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}
	}

	progressMessage, err = progressMessage.Edit(fmt.Sprintf(i18n("stickers.stickerStoled"), stickerSetShortName, stickerEmoji), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
	return err
}

func generateStickerSetName(message *telegram.NewMessage) (stickerSetShortName, stickerSetTitle string) {
	shortNamePrefix := "a_"
	shortNameSuffix := fmt.Sprintf("%d_by_%s", message.SenderID(), (message.Client.Me()).Username)

	var nameTitle string
	if senderUsername := message.Sender.Username; senderUsername != "" {
		nameTitle = "@" + senderUsername
	} else {
		nameTitle = message.Sender.FirstName
		if len(nameTitle) > 35 {
			nameTitle = nameTitle[:35]
		}
	}

	stickerSetTitle = fmt.Sprintf("%s's SmudgeLord", nameTitle)
	stickerSetShortName = shortNamePrefix + shortNameSuffix

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
	var err error

	file, err := os.Open(input)
	if err != nil {
		return err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	resizedImg := transform.Resize(img, 512, 512, transform.Lanczos)
	if err := os.Remove(input); err != nil {
		return err
	}

	err = imgio.Save(input, resizedImg, imgio.PNGEncoder())
	if err != nil {
		if err := os.Remove(input); err != nil {
			return err
		}
		return err
	}

	return nil
}

func convertVideo(inputFile string) (videoConverted string, err error) {
	defer func() {
		if err := os.Remove(inputFile); err != nil {
			log.Print(err)
		}
	}()
	outputFile, err := os.CreateTemp("", "Smudge*.webm")
	if err != nil {
		return videoConverted, err
	}

	cmd := exec.Command("ffmpeg",
		"-loglevel", "quiet", "-i", inputFile,
		"-t", "00:00:03", "-vf", "fps=30",
		"-c:v", "vp9", "-b:v", "500k",
		"-preset", "ultrafast", "-s", "512x512",
		"-y", "-f", "webm",
		outputFile.Name())

	err = cmd.Run()
	if err != nil {
		return videoConverted, err
	}
	_, err = outputFile.Seek(0, 0)
	if err != nil {
		return videoConverted, err
	}

	return outputFile.Name(), nil
}

func Load(client *telegram.Client) {
	utils.SotreHelp("stickers")
	client.On("command:getsticker", handlers.HandleCommand(handlerGetSticker))
	client.On("command:kang", handlers.HandleCommand(handlerKangSticker))

	handlers.DisableableCommands = append(handlers.DisableableCommands, "getsticker", "kang")
}
