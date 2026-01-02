package afk

import (
	"context"
	"database/sql"
	"log/slog"
	"regexp"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func CheckAFKMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		message := update.Message

		if message == nil {
			if update.CallbackQuery == nil {
				next(ctx, b, update)
				return
			}

			message = update.CallbackQuery.Message.Message

			if update.CallbackQuery.Message.Type == 1 || message == nil {
				next(ctx, b, update)
				return
			}
		}

		if message.From == nil ||
			message.Chat.Type != models.ChatTypeGroup &&
				message.Chat.Type != models.ChatTypeSupergroup ||
			regexp.MustCompile(`^(brb|/afk)`).MatchString(message.Text) {
			next(ctx, b, update)
			return
		}

		mentionedUserID := getUserIDFromMessage(message)
		if !userIsAway(mentionedUserID) {
			next(ctx, b, update)
			return
		}

		reason, duration, err := getUserAway(mentionedUserID)
		if err != nil && err != sql.ErrNoRows {
			slog.Error("Couldn't get user away status",
				"UserID", message.From.ID,
				"Error", err.Error())
			return
		}

		i18n := localization.Get(update)
		humanizedDuration := localization.HumanizeTimeSince(duration, update)

		switch mentionedUserID {
		case message.From.ID:
			if err := unsetUserAway(mentionedUserID); err != nil {
				slog.Error("Couldn't unset user away status",
					"UserID", mentionedUserID,
					"Error", err.Error())
				next(ctx, b, update)
				return
			}

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: message.Chat.ID,
				Text: i18n("user-now-available",
					map[string]any{
						"userID":        message.From.ID,
						"userFirstName": utils.EscapeHTML(message.From.FirstName),
						"duration":      humanizedDuration,
					}),
				LinkPreviewOptions: &models.LinkPreviewOptions{
					PreferLargeMedia: bot.True(),
				},
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: message.ID,
				},
			})
		default:
			user, err := b.GetChat(ctx, &bot.GetChatParams{ChatID: mentionedUserID})
			if err != nil {
				slog.Error("Couldn't get user",
					"UserID", mentionedUserID,
					"Error", err.Error())
				return
			}

			text := i18n("user-unavailable",
				map[string]any{
					"userID":        mentionedUserID,
					"userFirstName": utils.EscapeHTML(user.FirstName),
					"duration":      humanizedDuration,
				})

			if reason != "" {
				text += "\n" + i18n("user-unavailable-reason",
					map[string]any{
						"reason": reason,
					})
			}

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: message.Chat.ID,
				Text:   text,
				LinkPreviewOptions: &models.LinkPreviewOptions{
					PreferLargeMedia: bot.True(),
				},
				ParseMode: models.ParseModeHTML,
				ReplyParameters: &models.ReplyParameters{
					MessageID: message.ID,
				},
			})
		}
		next(ctx, b, update)
	}
}

func getUserIDFromMessage(message *models.Message) int64 {
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

				slog.Error("Couldn't get user ID from username",
					"Username", username,
					"Error", err.Error(),
				)
			}
		}
	}

	return message.From.ID
}

func setAFKHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	reason := extractReason(update.Message.Text)
	err := setUserAway(update.Message.From, reason, time.Now().UTC())
	if err != nil {
		slog.Error("Couldn't set user away status",
			"UserID", update.Message.From.ID,
			"Error", err.Error())
		return
	}

	i18n := localization.Get(update)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: i18n("user-now-unavailable",
			map[string]any{
				"userFirstName": utils.EscapeHTML(update.Message.From.FirstName),
			}),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func extractReason(text string) string {
	matches := regexp.MustCompile(`^(?:brb|\/afk)\s(.+)$`).FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeCommand, "afk", setAFKHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "^brb", setAFKHandler)

	utils.SaveHelp("afk")
}
