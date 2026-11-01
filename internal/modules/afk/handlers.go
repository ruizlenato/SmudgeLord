// Package afk implements the away-from-keyboard module for tracking user absence.
package afk

import (
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/referral"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var (
	afkPrefixRegex = regexp.MustCompile(`^(brb|/afk)`)
	mentionRegex   = regexp.MustCompile(`@[A-Za-z0-9_]{3,}`)
	afkReasonRegex = regexp.MustCompile(`^(?:brb|/afk)\s(.+)$`)
)

const afkMaxVideoDuration int64 = 30

func checkAFKMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	messageData := ctx.EffectiveMessage
	if messageData == nil || ctx.EffectiveUser == nil {
		return nil
	}

	if (messageData.Chat.Type != gotgbot.ChatTypeGroup && messageData.Chat.Type != gotgbot.ChatTypeSupergroup) ||
		afkPrefixRegex.MatchString(messageData.GetText()) {
		return nil
	}

	mentionedUserID := getUserIDFromMessage(messageData)
	if mentionedUserID == ctx.EffectiveUser.Id {
		mentionedUserID = 0
	}

	senderAway := userIsAway(ctx.EffectiveUser.Id)
	targetAway := mentionedUserID != 0 && userIsAway(mentionedUserID)
	if !senderAway && !targetAway {
		return nil
	}

	i18n := localization.Get(ctx)

	if senderAway {
		senderInfo, err := getUserAway(ctx.EffectiveUser.Id)
		if err != nil {
			slog.Error("Couldn't get user away status", "UserID", ctx.EffectiveUser.Id, "Error", err.Error())
			return nil
		}
		if err := unsetUserAway(ctx.EffectiveUser.Id); err != nil {
			slog.Error("Couldn't unset user away status", "UserID", ctx.EffectiveUser.Id, "Error", err.Error())
			return nil
		}

		_, _ = b.SendMessage(messageData.Chat.Id, i18n("user-now-available", map[string]any{
			"userID":        ctx.EffectiveUser.Id,
			"userFirstName": utils.EscapeHTML(ctx.EffectiveUser.FirstName),
			"duration":      localization.HumanizeTimeSince(senderInfo.duration, ctx),
		}), &gotgbot.SendMessageOpts{
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{PreferLargeMedia: true},
			ParseMode:          gotgbot.ParseModeHTML,
			ReplyParameters:    &gotgbot.ReplyParameters{MessageId: messageData.MessageId},
		})
	}

	if !targetAway {
		return nil
	}

	info, err := getUserAway(mentionedUserID)
	if err != nil {
		slog.Error("Couldn't get user away status", "UserID", mentionedUserID, "Error", err.Error())
		return nil
	}
	humanizedDuration := localization.HumanizeTimeSince(info.duration, ctx)

	member, err := b.GetChatMember(messageData.Chat.Id, mentionedUserID, nil)
	if err == nil {
		info.firstName = member.GetUser().FirstName
	}

	text := i18n("user-unavailable", map[string]any{
		"userID":        mentionedUserID,
		"userFirstName": utils.EscapeHTML(info.firstName),
		"duration":      humanizedDuration,
	})

	if info.reason != "" {
		text += "\n" + i18n("user-unavailable-reason", map[string]any{"reason": utils.EscapeHTML(info.reason)})
	}

	opts := &gotgbot.ReplyParameters{MessageId: messageData.MessageId}

	switch info.mediaType {
	case "photo":
		_, _ = b.SendPhoto(messageData.Chat.Id, gotgbot.InputFileByID(info.mediaID), &gotgbot.SendPhotoOpts{
			Caption:         text,
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: opts,
		})
	case "animation":
		_, _ = b.SendAnimation(messageData.Chat.Id, gotgbot.InputFileByID(info.mediaID), &gotgbot.SendAnimationOpts{
			Caption:         text,
			ParseMode:       gotgbot.ParseModeHTML,
			ReplyParameters: opts,
		})
	case "video":
		_, _ = b.SendVideo(messageData.Chat.Id, gotgbot.InputFileByID(info.mediaID), &gotgbot.SendVideoOpts{
			Caption:           text,
			ParseMode:         gotgbot.ParseModeHTML,
			ReplyParameters:   opts,
			SupportsStreaming: true,
		})
	default:
		_, _ = b.SendMessage(messageData.Chat.Id, text, &gotgbot.SendMessageOpts{
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{PreferLargeMedia: true},
			ParseMode:          gotgbot.ParseModeHTML,
			ReplyParameters:    &gotgbot.ReplyParameters{MessageId: messageData.MessageId},
		})
	}

	return nil
}

func getUserIDFromMessage(messageData *gotgbot.Message) int64 {
	if messageData.ReplyToMessage != nil && messageData.ReplyToMessage.From != nil {
		return messageData.ReplyToMessage.From.Id
	}

	for _, entity := range messageData.Entities {
		if entity.Type == "text_mention" && entity.User != nil {
			return entity.User.Id
		}
	}

	for _, entity := range messageData.CaptionEntities {
		if entity.Type == "text_mention" && entity.User != nil {
			return entity.User.Id
		}
	}

	for _, candidate := range extractMentionCandidates(messageData.GetText()) {
		userID, err := getIDFromUsername(candidate)
		if err == nil && userID != 0 {
			return userID
		}
	}

	for _, candidate := range extractMentionCandidates(messageData.Caption) {
		userID, err := getIDFromUsername(candidate)
		if err == nil && userID != 0 {
			return userID
		}
	}

	if messageData.From != nil {
		return messageData.From.Id
	}

	return 0
}

func extractMentionCandidates(text string) []string {
	if text == "" {
		return nil
	}
	matches := mentionRegex.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(matches))
	for _, m := range matches {
		if strings.TrimSpace(m) != "" {
			candidates = append(candidates, m)
		}
	}
	return candidates
}

func afkMediaFromMessage(messageData *gotgbot.Message) (mediaType, mediaID string, videoTooLong bool) {
	switch {
	case len(messageData.Photo) > 0:
		return "photo", messageData.Photo[len(messageData.Photo)-1].FileId, false
	case messageData.Animation != nil:
		return "animation", messageData.Animation.FileId, false
	case messageData.Video != nil:
		if messageData.Video.Duration > afkMaxVideoDuration {
			return "", "", true
		}
		return "video", messageData.Video.FileId, false
	}
	return "", "", false
}

func setAFKHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}

	i18n := localization.Get(ctx)

	reason := extractReason(ctx.EffectiveMessage.GetText())
	mediaType, mediaID, videoTooLong := afkMediaFromMessage(ctx.EffectiveMessage)
	if mediaType == "" && !videoTooLong && ctx.EffectiveMessage.ReplyToMessage != nil {
		mediaType, mediaID, videoTooLong = afkMediaFromMessage(ctx.EffectiveMessage.ReplyToMessage)
	}

	notice := ""
	mediaLocked := false
	switch {
	case videoTooLong:
		notice = i18n("afk-media-video-too-long")
	case mediaType != "" && !referral.IsAfkMediaUnlocked(ctx.EffectiveUser.Id):
		mediaLocked = true
		mediaType, mediaID = "", ""
		notice = i18n("afk-media-locked", map[string]any{"required": referral.RequiredAfkMediaReferrals})
	}

	media := ""
	if mediaType != "" {
		media = mediaType + ":" + mediaID
	}
	if err := setUserAway(ctx.EffectiveUser, reason, media, time.Now().UTC()); err != nil {
		slog.Error("Couldn't set user away status", "UserID", ctx.EffectiveUser.Id, "Error", err.Error())
		return nil
	}

	confirmation := i18n("user-now-unavailable", map[string]any{
		"userFirstName": utils.EscapeHTML(ctx.EffectiveUser.FirstName),
	})

	var replyMarkup *gotgbot.InlineKeyboardMarkup
	if notice != "" {
		confirmation += "\n\n" + notice
	}
	if mediaLocked {
		replyMarkup = &gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
				{
					Text: i18n("referral-unlock-button", map[string]any{
						"count":    referral.CountReferrals(ctx.EffectiveUser.Id),
						"required": referral.RequiredAfkMediaReferrals,
					}),
					CallbackData: "_refpnl",
				},
			}},
		}
	}

	opts := gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	}
	if replyMarkup != nil {
		opts.ReplyMarkup = replyMarkup
	}

	if _, err := b.SendMessage(ctx.EffectiveMessage.Chat.Id, confirmation, &opts); err != nil {
		slog.Error("Couldn't send AFK confirmation", "ChatID", ctx.EffectiveMessage.Chat.Id, "Error", err.Error())
	}

	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("afk", setAFKHandler))
	dispatcher.AddHandler(handlers.NewMessage(message.HasPrefix("brb"), setAFKHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, checkAFKMessage), -3)

	utils.SaveHelp("afk")
}

func extractReason(text string) string {
	matches := afkReasonRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
