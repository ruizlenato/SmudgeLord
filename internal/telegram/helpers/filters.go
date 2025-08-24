package helpers

import (
	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/config"
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

func IsBotOwner(message *telegram.NewMessage) bool {
	if message.SenderID() == config.OwnerID {
		return true
	}
	return false
}
