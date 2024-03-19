package modules

import (
	"regexp"

	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"
	"smudgelord/smudgelord/utils/medias"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func mediaDownloader(bot *telego.Bot, message telego.Message) {
	// Extract URL from the message text using regex
	url := regexp.MustCompile(`(?:htt.*?//)?(:?.*)?(?:instagram|twitter|x|tiktok|threads)\.(?:com|net)\/(?:\S*)`).FindStringSubmatch(message.Text)
	if len(url) < 1 {
		bot.SendMessage(telegoutil.Message(
			telegoutil.ID(message.Chat.ID),
			"No URL found",
		))
		return
	}

	dm := medias.NewDownloadMedia()
	mediaItems, caption := dm.Download(url[0])

	// Check if only one photo is present and link preview is enabled, then return
	if len(mediaItems) == 1 && mediaItems[0].MediaType() == "photo" && !message.LinkPreviewOptions.IsDisabled {
		return
	}

	if len(mediaItems) > 0 {
		for _, media := range mediaItems[:1] {
			switch media.MediaType() {
			case "photo":
				if photo, ok := media.(*telego.InputMediaPhoto); ok {
					photo.WithCaption(caption).WithParseMode("HTML")
				}
			case "video":
				if video, ok := media.(*telego.InputMediaVideo); ok {
					video.WithCaption(caption).WithParseMode("HTML")
				}
			}
		}
	}

	bot.SendMediaGroup(telegoutil.MediaGroup(
		telegoutil.ID(message.Chat.ID),
		mediaItems...,
	))
}

func mediaConfig(bot *telego.Bot, update telego.Update) {
	message := update.Message
	if message == nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	chat := message.GetChat()
	i18n := localization.Get(chat)

	if update.Message == nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(chat.ID),
			MessageID: update.CallbackQuery.Message.GetMessageID(),
			Text:      i18n("medias.config"),
			ParseMode: "HTML",
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      i18n("medias.config"),
			ParseMode: "HTML",
		})
	}
}

func LoadMediaDownloader(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("medias")
	bh.HandleMessage(mediaDownloader, telegohandler.TextMatches(regexp.MustCompile(`(?:htt.*?//)?(:?.*)?(?:instagram|twitter|x|tiktok|threads)\.(?:com|net)\/(?:\S*)`)))
	bh.Handle(mediaConfig, telegohandler.CallbackDataEqual("mediaConfig"), helpers.IsAdmin(bot))
}
