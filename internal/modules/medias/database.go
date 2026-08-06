package medias

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/ruizlenato/smudgelord/internal/database"
)

func isGroupLikeChat(chatType string) bool {
	return chatType == gotgbot.ChatTypeGroup || chatType == gotgbot.ChatTypeSupergroup
}

func getMediasAuto(chatID int64) bool {
	var mediasAuto bool
	if err := database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", chatID).Scan(&mediasAuto); err != nil || !mediasAuto {
		return false
	}
	return true
}

func getMediasCaption(chatID int64) bool {
	var mediasCaption bool
	if err := database.DB.QueryRow("SELECT mediasCaption FROM chats WHERE id = ?;", chatID).Scan(&mediasCaption); err != nil {
		return true
	}
	return mediasCaption
}

func getMediasErrors(chatID int64) bool {
	var mediasErrors bool
	if err := database.DB.QueryRow("SELECT mediasErrors FROM chats WHERE id = ?;", chatID).Scan(&mediasErrors); err != nil || !mediasErrors {
		return false
	}
	return true
}
