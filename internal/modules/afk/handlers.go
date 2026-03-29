package afk

import (
	"database/sql"
	"log/slog"
	"regexp"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func checkAFKMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	messageData := ctx.EffectiveMessage
	if messageData == nil || ctx.EffectiveUser == nil {
		return nil
	}

	if (messageData.Chat.Type != gotgbot.ChatTypeGroup && messageData.Chat.Type != gotgbot.ChatTypeSupergroup) ||
		regexp.MustCompile(`^(brb|/afk)`).MatchString(messageData.GetText()) {
		return nil
	}

	mentionedUserID := getUserIDFromMessage(messageData)
	if !userIsAway(mentionedUserID) {
		return nil
	}

	reason, duration, err := getUserAway(mentionedUserID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("Couldn't get user away status", "UserID", ctx.EffectiveUser.Id, "Error", err.Error())
		return nil
	}

	i18n := localization.Get(ctx)
	humanizedDuration := localization.HumanizeTimeSinceGotgbot(duration, ctx)

	if mentionedUserID == ctx.EffectiveUser.Id {
		if err := unsetUserAway(mentionedUserID); err != nil {
			slog.Error("Couldn't unset user away status", "UserID", mentionedUserID, "Error", err.Error())
			return nil
		}

		_, _ = b.SendMessage(messageData.Chat.Id, i18n("user-now-available", map[string]any{
			"userID":        ctx.EffectiveUser.Id,
			"userFirstName": utils.EscapeHTML(ctx.EffectiveUser.FirstName),
			"duration":      humanizedDuration,
		}), &gotgbot.SendMessageOpts{
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{PreferLargeMedia: true},
			ParseMode:          gotgbot.ParseModeHTML,
			ReplyParameters:    &gotgbot.ReplyParameters{MessageId: messageData.MessageId},
		})

		return nil
	}

	user, err := b.GetChat(mentionedUserID, nil)
	if err != nil {
		slog.Error("Couldn't get user", "UserID", mentionedUserID, "Error", err.Error())
		return nil
	}

	text := i18n("user-unavailable", map[string]any{
		"userID":        mentionedUserID,
		"userFirstName": utils.EscapeHTML(user.FirstName),
		"duration":      humanizedDuration,
	})

	if reason != "" {
		text += "\n" + i18n("user-unavailable-reason", map[string]any{"reason": reason})
	}

	_, _ = b.SendMessage(messageData.Chat.Id, text, &gotgbot.SendMessageOpts{
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{PreferLargeMedia: true},
		ParseMode:          gotgbot.ParseModeHTML,
		ReplyParameters:    &gotgbot.ReplyParameters{MessageId: messageData.MessageId},
	})

	return nil
}

func getUserIDFromMessage(messageData *gotgbot.Message) int64 {
	if messageData.ReplyToMessage != nil && messageData.ReplyToMessage.From != nil {
		return messageData.ReplyToMessage.From.Id
	}

	for _, entity := range messageData.Entities {
		if entity.Type != "mention" && entity.Type != "text_mention" {
			continue
		}

		if entity.Type == "text_mention" && entity.User != nil {
			return entity.User.Id
		}

		start := int(entity.Offset)
		end := int(entity.Offset + entity.Length)
		text := messageData.GetText()
		if start < 0 || end > len(text) || start >= end {
			continue
		}

		username := text[start:end]
		userID, err := getIDFromUsername(username)
		if err == nil && userID != 0 {
			return userID
		}
	}

	if messageData.From != nil {
		return messageData.From.Id
	}

	return 0
}

func setAFKHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}

	reason := extractReason(ctx.EffectiveMessage.GetText())
	if err := setUserAway(ctx.EffectiveUser, reason, time.Now().UTC()); err != nil {
		slog.Error("Couldn't set user away status", "UserID", ctx.EffectiveUser.Id, "Error", err.Error())
		return nil
	}

	i18n := localization.Get(ctx)
	_, _ = b.SendMessage(ctx.EffectiveMessage.Chat.Id, i18n("user-now-unavailable", map[string]any{
		"userFirstName": utils.EscapeHTML(ctx.EffectiveUser.FirstName),
	}), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	})

	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("afk", setAFKHandler))
	dispatcher.AddHandler(handlers.NewMessage(message.HasPrefix("brb"), setAFKHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, checkAFKMessage), 1)

	utils.SaveHelp("afk")
}

func extractReason(text string) string {
	matches := regexp.MustCompile(`^(?:brb|/afk)\s(.+)$`).FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
