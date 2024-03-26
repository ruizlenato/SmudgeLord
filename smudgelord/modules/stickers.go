package modules

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"

	"github.com/disintegration/imaging"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func getFileIDAndType(reply *telego.Message) (stickerAction string, stickerType string, fileID string) {
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

func getSticker(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message.Chat)
	if message.ReplyToMessage == nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("stickers.get_not_reply"),
			ParseMode: "HTML",
		})
		return
	}

	if replySticker := message.ReplyToMessage.Sticker; replySticker != nil && !replySticker.IsAnimated {
		file, err := bot.GetFile(&telego.GetFileParams{FileID: replySticker.FileID})
		if err != nil {
			log.Println(err)
			return
		}
		fileData, err := telegoutil.DownloadFile(bot.FileDownloadURL(file.FilePath))
		if err != nil {
			log.Println(err)
			return
		}
		stickerFile, err := bytesToFile(fileData)
		if err != nil {
			log.Println(err)
			bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				Text:      i18n("stickers.error"),
				ParseMode: "HTML",
			})
			return
		}

		bot.SendDocument(&telego.SendDocumentParams{
			ChatID:   telegoutil.ID(message.Chat.ID),
			Document: telegoutil.File(stickerFile),
		})
		defer stickerFile.Close()
	}
}

func kang(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message.Chat)
	if message.ReplyToMessage == nil {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("stickers.kang_not_reply"),
			ParseMode: "HTML",
		})
		return
	}
	progMSG, _ := bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      i18n("stickers.kanging"),
		ParseMode: "HTML",
	})

	stickerAction, stickerType, fileID := getFileIDAndType(message.ReplyToMessage)
	if stickerType == "" {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			MessageID: progMSG.GetMessageID(),
			Text:      i18n("stickers.invalid_type"),
			ParseMode: "HTML",
		})
		return
	}

	var (
		emoji           []string
		stickerSetName  string
		stickerSetTitle string // 1-64 characters
		stickerFile     *os.File
	)

	file, err := bot.GetFile(&telego.GetFileParams{FileID: fileID})
	if err != nil {
		log.Println(err)
		return
	}

	fileData, err := telegoutil.DownloadFile(bot.FileDownloadURL(file.FilePath))
	if err != nil {
		log.Println(err)
		return
	}

	if stickerAction == "resize" {
		stickerFile, err = resizeImage(fileData)
		if err != nil {
			log.Println(err)
			bot.EditMessageText(&telego.EditMessageTextParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				MessageID: progMSG.GetMessageID(),
				Text:      i18n("stickers.error"),
				ParseMode: "HTML",
			})
			return
		}
	} else if stickerAction == "convert" {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			MessageID: progMSG.GetMessageID(),
			Text:      i18n("stickers.converting"),
			ParseMode: "HTML",
		})
		stickerFile, err = convertVideo(fileData)
		if err != nil {
			log.Println(err)
			bot.EditMessageText(&telego.EditMessageTextParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				MessageID: progMSG.GetMessageID(),
				Text:      i18n("stickers.error"),
				ParseMode: "HTML",
			})
			return
		}
	} else {
		stickerFile, err = bytesToFile(fileData)
		if err != nil {
			log.Println(err)
			bot.EditMessageText(&telego.EditMessageTextParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				MessageID: progMSG.GetMessageID(),
				Text:      i18n("stickers.error"),
				ParseMode: "HTML",
			})
			return
		}

	}

	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	nameTitle := message.From.FirstName
	if len(nameTitle) > 35 {
		nameTitle = nameTitle[:35]
	}

	if message.From.Username != "" {
		nameTitle = "@" + message.From.Username
	}
	stickerSetTitle = nameTitle + "'s SmudgeLord"

	switch stickerType {
	case "video":
		stickerSetName = fmt.Sprintf("vid%d_by_%s", message.From.ID, botUser.Username)
		stickerSetTitle += " Video"
	case "animated":
		stickerSetName = fmt.Sprintf("anim%d_by_%s", message.From.ID, botUser.Username)
		stickerSetTitle += " Animated"
	case "static":
		stickerSetName = fmt.Sprintf("a%d_by_%s", message.From.ID, botUser.Username)
	}

	reEmoji := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{2694}-\x{2697}]|[\x{2702}-\x{27B0}]|[\x{1F926}-\x{1F937}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{2600}-\x{26FF}]`)
	emoji = append(emoji, reEmoji.FindAllString(message.Text, -1)...)

	if len(emoji) == 0 && message.ReplyToMessage.Sticker != nil {
		emoji = append(emoji, message.ReplyToMessage.Sticker.Emoji)
	}

	if len(emoji) == 0 {
		emoji = append(emoji, "🤔")
	}

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		MessageID: progMSG.GetMessageID(),
		Text:      i18n("stickers.pack_exists"),
		ParseMode: "HTML",
	})

	sticker := &telego.InputSticker{
		Sticker:   telegoutil.File(stickerFile),
		EmojiList: emoji,
	}

	err = bot.AddStickerToSet(&telego.AddStickerToSetParams{
		UserID:  message.From.ID,
		Name:    stickerSetName,
		Sticker: *sticker,
	})
	if err != nil {
		if strings.Contains(err.Error(), "STICKERSET_INVALID") {
			bot.EditMessageText(&telego.EditMessageTextParams{
				ChatID:    telegoutil.ID(message.Chat.ID),
				MessageID: progMSG.GetMessageID(),
				Text:      i18n("stickers.new_pack"),
				ParseMode: "HTML",
			})
			stickerFile.Seek(0, 0)
			bot.CreateNewStickerSet(&telego.CreateNewStickerSetParams{
				UserID:        message.From.ID,
				Name:          stickerSetName,
				Title:         stickerSetTitle,
				Stickers:      []telego.InputSticker{*sticker},
				StickerFormat: stickerType,
			})
		}
	}
	defer stickerFile.Close()

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		MessageID: progMSG.GetMessageID(),
		Text:      fmt.Sprintf(i18n("stickers.sticker_stoled"), stickerSetName, strings.Join(emoji, "")),
		ParseMode: "HTML",
	})
}

func bytesToFile(data []byte) (*os.File, error) {
	// Create a new temporary file with the .png extension
	tempFile, err := os.CreateTemp("", "*.png")
	if err != nil {
		log.Panic(err)
		return nil, err
	}

	_, err = tempFile.Write(data) // Write the byte slice to the file
	if err != nil {
		tempFile.Close()
		return nil, err
	}

	_, err = tempFile.Seek(0, 0) // Seek back to the beginning of the file
	if err != nil {
		tempFile.Close()
		return nil, err

	}

	return tempFile, nil // Return the file
}

func resizeImage(input []byte) (*os.File, error) {
	// Decode the input byte slice to an image
	img, err := imaging.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}

	// Resize the image to 512x512 pixels
	resizedImg := imaging.Resize(img, 512, 512, imaging.Lanczos)
	tempFile, err := os.CreateTemp("", "*.png")
	if err != nil {
		tempFile.Close()
		return nil, err
	}

	// Encode the resized image to the temporary file
	err = jpeg.Encode(tempFile, resizedImg, nil)
	if err != nil {
		tempFile.Close()
		return nil, err
	}

	_, err = tempFile.Seek(0, 0) // Seek back to the beginning of the file
	if err != nil {
		tempFile.Close()
		return nil, err
	}

	return tempFile, nil
}

// convertVideo converts a specified video in []byte format to a webm file.
// It returns an *os.File containing the converted video and an error, if any.
func convertVideo(input []byte) (*os.File, error) {
	// Create a temporary file for the input video
	inputFile, err := os.CreateTemp("", "input_*.mp4")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inputFile.Name()) // Remove the temporary input file when the function returns

	// Write the input video data to the temporary input file
	if _, err := inputFile.Write(input); err != nil {
		return nil, err
	}

	// Create a temporary file for the output video
	outputFile, err := os.CreateTemp("", "*.webm")
	if err != nil {
		return nil, err
	}

	// Run ffmpeg command to convert the input video to WebM format with specified settings
	cmd := exec.Command("ffmpeg",
		"-loglevel", "quiet", // Set log level to quiet
		"-i", inputFile.Name(), // Input file path
		"-t", "00:00:03", // Set duration of output video to 3 seconds
		"-vf", "fps=30", // Set frame rate to 30 fps
		"-c:v", "vp9", // Set video codec to VP9
		"-b:v", "500k", // Set video bitrate to 500k
		"-preset", "ultrafast", // Set preset to ultrafast for fast encoding
		"-s", "512x512", // Set output video resolution to 512x512
		"-y",         // Overwrite output file without asking
		"-an",        // Disable audio
		"-f", "webm", // Set output format to WebM
		outputFile.Name()) // Specify output file path

	err = cmd.Run() // Execute the ffmpeg command
	if err != nil {
		return nil, err
	}
	outputFile.Seek(0, 0) // Move the file pointer to the beginning of the output file

	return outputFile, nil // Return the converted video file
}

func LoadStickers(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("stickers")
	bh.HandleMessage(getSticker, telegohandler.CommandEqual("getsticker"))
	bh.HandleMessage(kang, telegohandler.CommandEqual("kang"))
}
