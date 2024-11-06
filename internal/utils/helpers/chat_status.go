package helpers

import (
	"log"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func IsAdmin(bot *telego.Bot) telegohandler.Predicate {
	return func(update telego.Update) bool {
		var message *telego.Message
		var userID int64

		switch {
		case update.Message != nil:
			message = update.Message
			userID = update.Message.From.ID
		case update.CallbackQuery != nil:
			if msg, ok := update.CallbackQuery.Message.(*telego.Message); ok {
				message = msg
				userID = update.CallbackQuery.From.ID
			} else {
				return false
			}
		default:
			return false
		}

		if message.Chat.Type == telego.ChatTypePrivate {
			return true
		}

		if message.SenderChat != nil {
			return false
		}

		chatMember, err := bot.GetChatMember(&telego.GetChatMemberParams{
			ChatID: telegoutil.ID(message.Chat.ID),
			UserID: userID,
		})
		if err != nil {
			log.Print("helpers/IsAdmin — Error getting chat member:", err)
			return false
		}

		status := chatMember.MemberStatus()
		return status == "creator" || status == "administrator"
	}
}

func IsGroup(update telego.Update) bool {
	message := update.Message
	if message == nil {
		if update.CallbackQuery == nil {
			return false
		}
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	return message.Chat.Type == telego.ChatTypeGroup || message.Chat.Type == telego.ChatTypeSupergroup
}
