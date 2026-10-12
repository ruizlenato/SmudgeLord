// Package referral implements the invite system used to unlock gated bot perks.
package referral

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	RequiredReferrals   = 10
	BigVideoDailyQuota  = 3
	BigVideoWeeklyQuota = 10
)

func Load(dispatcher *ext.Dispatcher) {
	// Runs in an earlier group than database.SaveUsers (group -1), so it can
	// detect whether the invited user is brand new to the bot.
	dispatcher.AddHandlerToGroup(handlers.NewCommand("start", captureReferral), -2)
	dispatcher.AddHandler(handlers.NewCommand("referral", referralHandler))
	dispatcher.AddHandler(handlers.NewCommand("convites", referralHandler))
	utils.SaveHelp("referral")
}

// Link returns the personal invite link of a user.
func Link(botUsername string, userID int64) string {
	return fmt.Sprintf("https://t.me/%s?start=ref_%d", botUsername, userID)
}

// IsUnlocked reports whether the user invited enough people to unlock gated perks.
func IsUnlocked(userID int64) bool {
	return countReferrals(userID) >= RequiredReferrals
}

// CountReferrals returns how many users a user has invited.
func CountReferrals(userID int64) int {
	return countReferrals(userID)
}

func startPayload(ctx *ext.Context) (string, bool) {
	if ctx.EffectiveMessage == nil {
		return "", false
	}
	fields := strings.Fields(ctx.EffectiveMessage.GetText())
	if len(fields) < 2 {
		return "", false
	}
	return strings.CutPrefix(fields[1], "ref_")
}

// captureReferral credits the inviter when a brand new user starts the bot
// through an invite link (/start ref_<id>). It runs silently, before the
// database saves the new user, and never blocks the normal /start flow.
func captureReferral(b *gotgbot.Bot, ctx *ext.Context) error {
	payload, found := startPayload(ctx)
	if !found || ctx.EffectiveUser == nil {
		return nil
	}

	referredID := ctx.EffectiveUser.Id
	referrerID, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || referrerID == referredID {
		return nil
	}

	referredRegistered, err := isRegistered(referredID)
	if err != nil {
		slog.Error("Couldn't verify invited user", "ReferredID", referredID, "Error", err.Error())
		return nil
	}
	alreadyReferred, err := isReferred(referredID)
	if err != nil {
		slog.Error("Couldn't check referral", "ReferredID", referredID, "Error", err.Error())
		return nil
	}
	referrerRegistered, err := isRegistered(referrerID)
	if err != nil {
		slog.Error("Couldn't verify referrer", "ReferrerID", referrerID, "Error", err.Error())
		return nil
	}
	if referredRegistered || alreadyReferred || !referrerRegistered {
		return nil
	}

	if err := registerReferral(referrerID, referredID); err != nil {
		slog.Error("Couldn't register referral", "ReferrerID", referrerID, "ReferredID", referredID, "Error", err.Error())
		return nil
	}

	notifyReferrer(b, referrerID, ctx.EffectiveUser.FirstName)
	return nil
}

func notifyReferrer(b *gotgbot.Bot, referrerID int64, referredName string) {
	i18n := localization.GetForUser(referrerID)
	count := countReferrals(referrerID)

	key := "referral-notification"
	if count >= RequiredReferrals {
		key = "referral-unlocked-notification"
	}

	_, err := b.SendMessage(referrerID, i18n(key, map[string]any{
		"name":     html.EscapeString(referredName),
		"count":    count,
		"required": RequiredReferrals,
	}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	if err != nil {
		slog.Debug("Couldn't notify referrer", "ReferrerID", referrerID, "Error", err.Error())
	}
}

func referralHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	sendPanel(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveUser.Id, localization.Get(ctx))
	return nil
}

// ShowPanel shows the invite panel when a user opens their own invite link
// (/start ref_<own id>). It reports whether the panel was shown.
func ShowPanel(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return false
	}
	if ctx.EffectiveMessage.Chat.Type != gotgbot.ChatTypePrivate {
		return false
	}

	payload, found := startPayload(ctx)
	if !found {
		return false
	}
	ownID, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || ownID != ctx.EffectiveUser.Id {
		return false
	}

	sendPanel(b, ctx.EffectiveMessage.Chat.Id, ctx.EffectiveUser.Id, localization.Get(ctx))
	return true
}

func sendPanel(b *gotgbot.Bot, chatID, userID int64, i18n func(string, ...map[string]any) string) {
	count := countReferrals(userID)
	link := Link(b.User.Username, userID)

	body := utils.RichHTML(i18n("referral-message"))
	body += "\n<hr/>\n" + utils.RichHTML(i18n("referral-invites", map[string]any{
		"count":    count,
		"required": RequiredReferrals,
	}))
	body += "\n" + utils.RichMessageButtonRow(
		utils.RichMessageButton(i18n("referral-copy-button"), "primary", "copy_text", "text", link),
		utils.RichMessageButton(i18n("referral-share-button"), "primary", "url", "url", "https://t.me/share/url?url="+url.QueryEscape(link)),
	)

	_, err := b.SendRichMessageWithContext(context.Background(), chatID, gotgbot.InputRichMessage{Html: body}, nil)
	if err != nil {
		slog.Error("Couldn't send referral panel", "ChatID", chatID, "Error", err.Error())
	}
}
