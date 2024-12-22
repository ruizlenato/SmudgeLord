package afk

import (
	"database/sql"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func checkAFK(bot *telego.Bot, update telego.Update, next telegohandler.Handler) {
	message := update.Message
	if message == nil && update.CallbackQuery != nil {
		switch msg := update.CallbackQuery.Message.(type) {
		case *telego.Message:
			message = msg
		default:
			next(bot, update)
			return
		}
	} else if message == nil ||
		message.From == nil ||
		!strings.Contains(message.Chat.Type, "group") ||
		regexp.MustCompile(`^/\bafk\b|^\bbrb\b`).MatchString(message.Text) {
		next(bot, update)
		return
	}

	user_id := getUserIDFromMessage(message)
	if user_id == 0 || !user_is_away(user_id) {
		next(bot, update)
		return
	}

	reason, duration, err := get_user_away(user_id)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Couldn't get user away status", "UserID", user_id, "Error", err.Error())
		return
	}

	i18n := localization.Get(update)

	humanizedDuration := localization.HumanizeTimeSince(duration, update)

	switch user_id {
	case message.From.ID:
		if err = unset_user_away(user_id); err != nil {
			slog.Error("CoCouldn't unset user away status", "UserID", user_id, "Error", err.Error())
			return
		}
		bot.SendMessage(&telego.SendMessageParams{
			ChatID: telegoutil.ID(message.Chat.ID),
			Text: i18n("now-available",
				map[string]interface{}{
					"userID":        message.From.ID,
					"userFirstName": message.From.FirstName,
					"duration":      humanizedDuration,
				}),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
	default:
		user, err := bot.GetChat(&telego.GetChatParams{ChatID: telegoutil.ID(user_id)})
		if err != nil {
			slog.Error("Couldn't get user", "UserID", user_id, "Error", err.Error())
			return
		}

		text := i18n("user-unavailable",
			map[string]interface{}{
				"userID":        user_id,
				"userFirstName": user.FirstName,
				"duration":      humanizedDuration,
			})

		if reason != "" {
			text += "\n" + i18n("user-unavailable-reason",
				map[string]interface{}{
					"reason": reason,
				})
		}

		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      text,
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
	}

	next(bot, update)
}

func handleSetAFK(bot *telego.Bot, message telego.Message) {
	reason := extractReason(message.Text)
	err := set_user_away(message.From.ID, reason, time.Now().UTC())
	if err != nil {
		slog.Error("Couldn't set user away status", "UserID", message.From.ID, "Error", err.Error())
		return
	}

	i18n := localization.Get(message)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Text: i18n("user-now-unavailable",
			map[string]interface{}{
				"userFirstName": utils.EscapeHTML(message.From.FirstName),
			}),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func getUserIDFromMessage(message *telego.Message) int64 {
	if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		return message.ReplyToMessage.From.ID
	}

	if message.Entities != nil {
		for _, entity := range message.Entities {
			if entity.Type == "mention" || entity.Type == "text_mention" {
				if entity.Type == "text_mention" {
					return entity.User.ID
				}
				username := message.Text[entity.Offset : entity.Offset+entity.Length]
				userID, err := getIDFromUsername(username)
				if err == nil {
					return userID
				}
				slog.Error("Couldn't get user ID from username", "Username", username, "Error", err.Error())
			}
		}
	}

	return message.From.ID
}

func extractReason(text string) string {
	matches := regexp.MustCompile(`^(?:brb|\/afk)\s(.+)$`).FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("afk")
	bh.Use(checkAFK)
	bh.HandleMessage(handleSetAFK, telegohandler.Or(
		telegohandler.CommandEqual("afk"),
		telegohandler.TextMatches(regexp.MustCompile(`^(?:brb)(\s.+)?`)),
	))
}
