package afk

import (
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"

	"github.com/amarnathcjd/gogram/telegram"
)

func checkAFK(message *telegram.NewMessage) error {
	if message.ChatType() == "user" {
		return nil
	}

	match, err := regexp.MatchString(`^(brb|/afk)`, strings.Split(message.Text(), " ")[0])
	if err != nil || match {
		return err
	}

	userID, err := getUserIDFromMessage(message)
	if err != nil {
		return err
	}

	isAway, err := userIsAway(userID)
	if err != nil || !isAway {
		return err
	}

	reason, duration, err := getUserAway(userID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	i18n := localization.Get(message)
	humanizedDuration := localization.HumanizeTimeSince(duration, message)

	switch userID {
	case message.Sender.ID:
		if err := unsetUserAway(userID); err != nil {
			return err
		}

		_, err := message.Reply(i18n("user-now-available",
			map[string]any{
				"userID":        message.Sender.ID,
				"userFirstName": message.Sender.FirstName,
				"duration":      humanizedDuration,
			}),
			telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
		return err
	default:
		user, err := message.Client.GetUser(userID)
		if err != nil {
			return err
		}

		text := i18n("user-unavailable",
			map[string]any{
				"userID":        userID,
				"userFirstName": user.FirstName,
				"duration":      humanizedDuration,
			})
		if reason != "" {
			text += "\n" + i18n("user-unavailable-reason",
				map[string]any{
					"reason": reason,
				})
		}

		_, err = message.Reply(text, telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}
}

func handlerSetAFK(message *telegram.NewMessage) error {
	err := setUserAway(message.Sender.ID, message.Args(), time.Now().UTC())
	if err != nil {
		return err
	}

	i18n := localization.Get(message)
	_, err = message.Reply(i18n("user-now-unavailable",
		map[string]any{
			"userFirstName": message.Sender.FirstName,
		}),
		telegram.SendOptions{
			ParseMode: telegram.HTML,
		})

	return err
}

func getUserIDFromMessage(message *telegram.NewMessage) (int64, error) {
	if message.IsReply() {
		reply, err := message.GetReplyMessage()
		if err != nil {
			return 0, err
		}
		return reply.Sender.ID, nil
	}

	if message.Message.Entities != nil {
		for _, entity := range message.Message.Entities {
			switch entity := entity.(type) {
			case *telegram.MessageEntityMentionName:
				return entity.UserID, nil
			case *telegram.MessageEntityMention:
				username := message.Text()[entity.Offset : entity.Offset+entity.Length]
				userID, err := getIDFromUsername(username)
				if err != nil {
					return 0, err
				}
				return userID, nil
			}
		}
	}
	return message.SenderID(), nil
}

func Load(client *telegram.Client) {
	utils.SotreHelp("afk")
	client.On(telegram.OnMessage, checkAFK)
	client.On("command:afk", handlers.HandleCommand(handlerSetAFK))
	client.On("message:^brb", handlers.HandleCommand(handlerSetAFK))

	handlers.DisableableCommands = append(handlers.DisableableCommands, "afk", "brb")
}
