package afk

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func checkAFK(bot *telego.Bot, update telego.Update, next telegohandler.Handler) {
	// Get the message from the update.
	message := update.Message
	if message == nil && update.CallbackQuery != nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	} else if message == nil ||
		message.From == nil ||
		!strings.Contains(message.Chat.Type, "group") ||
		regexp.MustCompile(`^/\bafk\b|^\bbrb\b`).MatchString(message.Text) {
		next(bot, update) // Call the next handler in the processing chain
		return
	}

	// Check if the user is away.
	user_id := getUserIDFromMessage(message)
	if user_id == 0 || !user_is_away(user_id) {
		next(bot, update) // Call the next handler in the processing chain
		return
	}

	reason, duration, err := get_user_away(user_id)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return
	}

	i18n := localization.Get(message.Chat)
	humanizedDuration := localization.HumanizeTimeSince(duration, message.Chat)

	switch {
	case user_id == message.From.ID:
		if err = unset_user_away(user_id); err != nil {
			log.Print(err)
			return
		}
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      fmt.Sprintf(i18n("afk.nowAvailable"), message.From.ID, message.From.FirstName, humanizedDuration),
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
			log.Printf("[afk/getChat] Error getting user: %v", err)
			return
		}

		text := fmt.Sprintf(i18n("afk.unavailable"), user_id, user.FirstName, humanizedDuration)
		if reason != "" {
			text += fmt.Sprintf(i18n("afk.reason"), reason)
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

	// Call the next handler in the processing chain.
	next(bot, update)
}

// setAwayCommand sets the AFK status for the user who sent the command.
func handleSetAFK(bot *telego.Bot, message telego.Message) {
	reason := extractReason(message.Text)
	err := set_user_away(message.From.ID, reason, time.Now().UTC())
	if err != nil {
		log.Print("[afk/setAFK] Error inserting user: ", err)
		return
	}

	i18n := localization.Get(message.Chat)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf(i18n("afk.nowUnavailable"), message.From.FirstName),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

// getUserIDFromMessage extracts the user ID from a message, handling mentions and replies.
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
				log.Printf("Error getting user ID from username: %v", err)
			}
		}
	}

	return message.From.ID
}

// extractReason extracts the reason from the AFK command message.
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
