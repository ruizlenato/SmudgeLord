package modules

import (
	"fmt"
	"log"
	"regexp"
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func getAFK(userID int64) (int64, string, time.Duration, error) {
	var id int64
	var reason string
	var afkTime time.Time
	rows, err := database.DB.Query("SELECT * FROM afk WHERE id = ?;", userID)
	if err != nil {
		return id, reason, time.Since(afkTime), err
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&id, &reason, &afkTime)
		if err != nil {
			return id, reason, time.Since(afkTime), err
		}
	}
	return id, reason, time.Since(afkTime), nil
}

func unsetAFK(userID int64) error {
	query := "DELETE FROM afk WHERE id = ?;"
	_, err := database.DB.Exec(query, userID)
	if err != nil {
		return err
	}
	return nil
}

func CheckAFK(bot *telego.Bot, update telego.Update, next telegohandler.Handler) {
	if !strings.Contains(update.Message.Chat.Type, "group") || update.Message.From == nil {
		next(bot, update)
		return
	}
	i18n := localization.Get(update.Message.Chat)
	userID := update.Message.From.ID

	if update.Message.Entities != nil {
		for _, entity := range update.Message.Entities {
			if strings.Contains(entity.Type, "mention") {
				if entity.Type == "text_mention" {
					userID = entity.User.ID
					break
				}
				row := database.DB.QueryRow(`SELECT id FROM users WHERE username = $1`, update.Message.Text[entity.Offset:entity.Offset+entity.Length])
				if row.Scan(&userID); row.Err() != nil {
					panic(row.Err())
				}
				break
			}
		}
	} else if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From != nil {
		userID = update.Message.ReplyToMessage.From.ID
	}

	id, reason, duration, err := getAFK(userID)
	if err != nil {
		log.Panic(err)
	}
	humanizedDuration := localization.HumanizeTimeSince(duration, update.Message.Chat)

	switch {
	case id == update.Message.From.ID:
		err = unsetAFK(id)
		if err != nil {
			log.Panic(err)
		}
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      fmt.Sprintf(i18n("afk.now-available"), update.Message.From.ID, update.Message.From.FirstName, humanizedDuration),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: update.Message.MessageID,
			},
		})
	case id != 0:
		user, err := bot.GetChat(&telego.GetChatParams{ChatID: telegoutil.ID(id)})
		if err != nil {
			log.Panic(err)
		}

		text := fmt.Sprintf(i18n("afk.unavailable"), id, user.FirstName, humanizedDuration)
		if reason != "" {
			text += fmt.Sprintf(i18n("afk.reason"), reason)
		}

		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      text,
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: update.Message.MessageID,
			},
		})
	}

	// Call the next handler in the processing chain.
	next(bot, update)
}

func SetAFK(bot *telego.Bot, message telego.Message) {
	var reason string

	matches := regexp.MustCompile(`^(?:brb|\/afk)\s(.+)$`).FindStringSubmatch(message.Text)
	if len(matches) > 1 {
		reason = matches[1]
	}

	query := "INSERT OR IGNORE INTO afk (id, reason, time) VALUES (?, ?, ?);"
	_, err := database.DB.Exec(query, message.From.ID, reason, time.Now().UTC())
	if err != nil {
		log.Print("Error inserting user:", err)
		return
	}

	i18n := localization.Get(message.Chat)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf(i18n("afk.now-unavailable"), message.From.FirstName),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}
