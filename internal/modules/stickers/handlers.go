package stickers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/h2non/bimg"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
)

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

	_, stickerType, emoji := extractStickerInfo(reply)
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

	_, err = message.ReplyMedia(stickerFile, telegram.MediaOptions{ForceDocument: true, Caption: fmt.Sprintf("<b>Emoji:</b> %s", emoji)})
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

	progressMsg, err := message.Reply(i18n("stickers.kanging"), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
	if err != nil {
		return err
	}

	reply, err := message.GetReplyMessage()
	if err != nil {
		return err
	}

	stickerAction, stickerType, emoji := extractStickerInfo(reply)
	if emoji == "" {
		emoji = "🤔"
	}
	if stickerType == "" {
		progressMsg.Edit(i18n("stickers.invalidType"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
	}

	var filename string
	switch stickerType {
	case "static":
		filename = "Smudge*.png"
	case "animated":
		filename = "Smudge*.tgs"
	case "video":
		filename = "Smudge*.webm"
	}
	file, err := os.CreateTemp("", filename)
	if err != nil {
		return err
	}

	stickerFile, err := reply.Download(&telegram.DownloadOptions{FileName: file.Name()})
	if err != nil {
		return err
	}

	switch stickerAction {
	case "resize":
		stickerFile, err = resizeImage(stickerFile)
		if err != nil {
			progressMsg.Edit(i18n("stickers.error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}
	case "convert":
		stickerFile, err = convertVideo(stickerFile)
		if err != nil {
			progressMsg.Edit(i18n("stickers.error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}
		progressMsg.Edit(i18n("stickers.converting"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
	}

	defer os.Remove(stickerFile)
	stickerSetName, stickerSetTitle := generateStickerSetName(message, stickerType)

	mediaMsg, err := message.Client.SendMedia(config.ChannelLogID, stickerFile, &telegram.MediaOptions{
		ForceDocument: true,
	})
	if err != nil {
		progressMsg.Edit(i18n("stickers.error"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}
	defer mediaMsg.Delete()

	progressMsg.Edit(i18n("stickers.packExists"), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
	_, err = message.Client.StickersAddStickerToSet(&telegram.InputStickerSetShortName{ShortName: stickerSetName}, &telegram.InputStickerSetItem{
		Document: &telegram.InputDocumentObj{ID: mediaMsg.Document().ID, AccessHash: mediaMsg.Document().AccessHash},
		Emoji:    emoji,
	})
	if err != nil {
		progressMsg.Edit(i18n("stickers.newPack"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		_, err = message.Client.StickersCreateStickerSet(&telegram.StickersCreateStickerSetParams{
			UserID:    &telegram.InputUserObj{UserID: message.Sender.ID, AccessHash: message.Sender.AccessHash},
			ShortName: stickerSetName,
			Title:     stickerSetTitle,
			Stickers: []*telegram.InputStickerSetItem{{
				Document: &telegram.InputDocumentObj{ID: mediaMsg.Document().ID, AccessHash: mediaMsg.Document().AccessHash},
				Emoji:    emoji,
			}},
		})
		if err != nil {
			progressMsg.Edit(i18n("stickers.error"), telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}
	}

	_, err = progressMsg.Edit(fmt.Sprintf(i18n("stickers.stickerStoled"), stickerSetName, emoji), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})
	return err
}

func generateStickerSetName(message *telegram.NewMessage, stickerType string) (stickerSetName, stickerSetTitle string) {
	packSuffix := fmt.Sprintf("%d_by_%s", message.SenderID(), message.Client.Me().Username)
	nameTitle := message.Sender.FirstName
	if len(nameTitle) > 35 {
		nameTitle = nameTitle[:35]
	}
	if senderUsername := message.Sender.Username; senderUsername != "" {
		nameTitle = "@" + senderUsername
	}

	stickerSetTitle = nameTitle + "'s SmudgeLord"
	var packPrefix string
	switch stickerType {
	case "video":
		packPrefix = "vid_"
		stickerSetTitle += " Video"
	case "animated":
		packPrefix = "anim_"
		stickerSetTitle += " Animated"
	case "static":
		packPrefix = "a_"
	}

	stickerSetName = packPrefix + packSuffix
	for i := 0; checkStickerSetCount(message, stickerSetName); i++ {
		stickerSetName = fmt.Sprintf("%s%d_%s", packPrefix, i, packSuffix)
	}
	return stickerSetName, stickerSetTitle
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

func extractStickerInfo(reply *telegram.NewMessage) (stickerAction string, stickerType string, emoji string) {
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
		return stickerAction, stickerType, emoji
	}

	if reply.IsMedia() {
		if _, ok := reply.Media().(*telegram.MessageMediaPhoto); ok {
			stickerType = "static"
			stickerAction = "resize"
		}
		if media, ok := reply.Media().(*telegram.MessageMediaDocument); ok {
			if document, ok := media.Document.(*telegram.DocumentObj); ok {
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
			}
		}
		return stickerAction, stickerType, emoji
	}
	return stickerAction, stickerType, emoji
}

func resizeImage(input string) (imageResized string, err error) {
	defer os.Remove(input)
	buffer, err := bimg.Read(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	resizedImg, err := bimg.Resize(buffer, bimg.Options{
		Width: 512, Height: 512, Quality: 100,
	})
	if err != nil {
		return imageResized, err
	}

	tempFile, err := os.CreateTemp("", "Smudge*.png")
	if err != nil {
		return imageResized, err
	}

	defer tempFile.Close()

	_, err = tempFile.Write(resizedImg)
	if err != nil {
		return imageResized, err
	}

	defer func() {
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}
	}()

	_, err = tempFile.Seek(0, 0)
	if err != nil {
		return imageResized, err
	}

	return tempFile.Name(), nil
}

func convertVideo(inputFile string) (videoConverted string, err error) {
	defer os.Remove(inputFile)

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
	outputFile.Seek(0, 0)

	return outputFile.Name(), nil
}

func Load(client *telegram.Client) {
	client.On("command:getsticker", handlers.HandleCommand(handlerGetSticker))
	client.On("command:kang", handlers.HandleCommand(handlerKangSticker))

	handlers.DisableableCommands = append(handlers.DisableableCommands, "getsticker", "kang")
}
