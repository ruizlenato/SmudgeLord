package helpers

import (
	"github.com/amarnathcjd/gogram/telegram"
)

func IsAdmin(message *telegram.NewMessage) bool {
	if message.ChatType() == telegram.EntityUser {
		return true
	}

	telegramParticipant, err := message.Client.GetChatMember(message.ChatID(), message.SenderID())
	if err != nil {
		return false
	}

	if telegramParticipant.Status == telegram.Admin || telegramParticipant.Status == telegram.Creator {
		return true
	}

	return false
}
