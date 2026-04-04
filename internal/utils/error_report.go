package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const SupportGroupHandle = "@SmudgeLordGroup"
const SupportGroupURL = "https://t.me/SmudgeLordGroup"

func NewUserErrorID(userID int64) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		fallback := fmt.Sprintf("%x", time.Now().UnixNano())
		if len(fallback) > 8 {
			fallback = fallback[len(fallback)-8:]
		}
		return fmt.Sprintf("%d-%s", userID, strings.ToUpper(fallback))
	}

	return fmt.Sprintf("%d-%s", userID, strings.ToUpper(hex.EncodeToString(buf)))
}

func ErrorI18nArgs(errorID string) map[string]any {
	return map[string]any{
		"errorId":      errorID,
		"supportGroup": SupportGroupHandle,
	}
}

func BuildErrorReportMessage(i18n func(string, ...map[string]any) string, summaryKey, errorID string) string {
	args := ErrorI18nArgs(errorID)
	args["summary"] = i18n(summaryKey)
	return i18n("error-report", args)
}

func BuildErrorReportAlert(i18n func(string, ...map[string]any) string, summaryKey, errorID string) string {
	args := ErrorI18nArgs(errorID)
	args["summary"] = i18n(summaryKey)
	return i18n("error-report-alert", args)
}

func ErrorReportKeyboard(i18n func(string, ...map[string]any) string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
		Text: i18n("error-report-button"),
		Url:  SupportGroupURL,
	}}}}
}

func LogErrorWithID(message, errorID string, err error, attrs ...any) {
	fields := []any{"error_id", errorID}
	if err != nil {
		fields = append(fields, "error", err.Error())
	}
	fields = append(fields, attrs...)

	ctx := context.Background()
	handler := slog.Default().Handler()
	if !handler.Enabled(ctx, slog.LevelError) {
		return
	}

	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		pc = 0
	}

	record := slog.NewRecord(time.Now(), slog.LevelError, message, pc)
	record.Add(fields...)
	_ = handler.Handle(ctx, record)
}
