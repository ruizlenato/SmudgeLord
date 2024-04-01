package modules

import (
	"fmt"
	"strings"

	"smudgelord/smudgelord/config"
	"smudgelord/smudgelord/database"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func announce(bot *telego.Bot, message telego.Message) {
	if message.From.ID != config.OWNER_ID {
		return
	}
	messageFields := strings.Fields(message.Text)
	if len(messageFields) < 2 {
		return
	}

	announceType := messageFields[1]
	announceText := strings.Join(strings.Fields(message.Text)[2:], " ")
	var query string

	fmt.Println(announceType)

	switch announceType {
	case "groups":
		query = "SELECT id FROM groups;"
	case "users":
		query = "SELECT id FROM users;"
	default:
		query = "SELECT id FROM users UNION ALL SELECT id FROM groups;"
		announceText = strings.Join(messageFields[1:], " ")
		if len(messageFields) > 2 {
			announceText = strings.Join(messageFields[2:], " ")
		}
	}

	rows, err := database.DB.Query(query)
	if err != nil {
		return
	}
	defer rows.Close()

	var successCount, errorCount int

	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			continue
		}

		_, err := bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(chatID),
			Text:      announceText,
			ParseMode: "HTML",
		})
		if err != nil {
			errorCount++
			continue
		}

		successCount++
	}

	if err := rows.Err(); err != nil {
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(config.OWNER_ID),
		Text:      fmt.Sprintf("<b>Messages sent successfully:</b> <code>%d</code>\n<b>Messages unsent:</b> <code>%d</code>", successCount, errorCount),
		ParseMode: "HTML",
	})
}

func LoadSudoers(bh *telegohandler.BotHandler, bot *telego.Bot) {
	bh.HandleMessage(announce, telegohandler.CommandEqual("announce"))
}
