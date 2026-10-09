package referral

import (
	"fmt"
	"log/slog"

	"github.com/ruizlenato/smudgelord/internal/database"
)

func isBotUser(userID int64) bool {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&exists)
	if err != nil {
		slog.Error("Couldn't check if user exists", "UserID", userID, "Error", err.Error())
		return false
	}
	return exists
}

func isReferred(referredID int64) bool {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM referrals WHERE referred_id = ?)", referredID).Scan(&exists)
	if err != nil {
		slog.Error("Couldn't check referral", "ReferredID", referredID, "Error", err.Error())
		return true
	}
	return exists
}

func countReferrals(referrerID int64) int {
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM referrals WHERE referrer_id = ?", referrerID).Scan(&count)
	if err != nil {
		slog.Error("Couldn't count referrals", "ReferrerID", referrerID, "Error", err.Error())
		return 0
	}
	return count
}

func registerReferral(referrerID, referredID int64) error {
	_, err := database.DB.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", referrerID, referredID)
	if err != nil {
		return fmt.Errorf("error registering referral: %w", err)
	}
	return nil
}

func bigVideoDownloadsSince(userID int64, window string) int {
	var count int
	err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM big_video_downloads WHERE user_id = ? AND created_at >= datetime('now', ?)",
		userID, window,
	).Scan(&count)
	if err != nil {
		slog.Error("Couldn't count big video downloads", "UserID", userID, "Error", err.Error())
		return 0
	}
	return count
}

func BigVideoQuotaRemaining(userID int64) (daily, weekly int) {
	return BigVideoDailyQuota - bigVideoDownloadsSince(userID, "-1 day"),
		BigVideoWeeklyQuota - bigVideoDownloadsSince(userID, "-7 days")
}

func CanDownloadBigVideo(userID int64) bool {
	if !IsUnlocked(userID) {
		return false
	}
	daily, weekly := BigVideoQuotaRemaining(userID)
	return daily > 0 && weekly > 0
}

func RegisterBigVideoDownload(userID int64) error {
	_, err := database.DB.Exec("INSERT INTO big_video_downloads (user_id) VALUES (?)", userID)
	if err != nil {
		return fmt.Errorf("error registering big video download: %w", err)
	}
	return nil
}
