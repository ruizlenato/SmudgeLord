package helpers

import (
	"log"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func IsAdmin(bot *telego.Bot) telegohandler.Predicate {
	return func(update telego.Update) bool {
		message := update.Message

		if message == nil {
			if update.CallbackQuery == nil {
				return false
			}
			message = update.CallbackQuery.Message.(*telego.Message)
		}

		// If the message is sent by a private chat, return without further processing.
		if message.Chat.Type == telego.ChatTypePrivate {
			return true
		}

		// If the message is sent by the sender's chat (e.g., channels or anonymous users), return without further processing.
		if message.SenderChat != nil {
			return false
		}

		userID := message.From.ID
		if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
		}

		chatMember, err := bot.GetChatMember(&telego.GetChatMemberParams{
			ChatID: telegoutil.ID(message.Chat.ID),
			UserID: userID,
		})
		if err != nil {
			log.Println(err)
			return false
		}

		if chatMember.MemberStatus() == "creator" || chatMember.MemberStatus() == "administrator" {
			return true
		}

		return false
	}
}
