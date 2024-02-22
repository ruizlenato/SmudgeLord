package modules

import (
	"fmt"
	"log"
	"smudgelord/smudgelord/localization"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

func Start(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update.Message.Chat)

	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	if strings.Contains(update.Message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      fmt.Sprintf(i18n("start_message_group"), botUser.FirstName),
			ParseMode: "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("start_button"),
					URL:  fmt.Sprintf("https://t.me/%s?start=start", botUser.Username),
				})),
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      fmt.Sprintf(i18n("start_message_private"), update.Message.From.FirstName, botUser.FirstName),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
		})
	}
}
