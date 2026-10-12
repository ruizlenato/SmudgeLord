package referral

import (
	"fmt"
	"log/slog"

	"github.com/ruizlenato/smudgelord/internal/database"
)

func isRegistered(userID int64) (bool, error) {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("couldn't check if user is registered: %w", err)
	}
	return exists, nil
}

func isReferred(referredID int64) (bool, error) {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM referrals WHERE referred_id = ?)", referredID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("couldn't check if user is referred: %w", err)
	}
	return exists, nil
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

func ReserveBigVideoDownload(userID int64) (reservationID int64, ok bool, err error) {
	res, err := database.DB.Exec(`
		INSERT INTO big_video_downloads (user_id)
		SELECT ?
		WHERE (SELECT COUNT(*) FROM referrals WHERE referrer_id = ?) >= ?
		  AND (SELECT COUNT(*) FROM big_video_downloads WHERE user_id = ? AND created_at >= datetime('now', '-1 day')) < ?
		  AND (SELECT COUNT(*) FROM big_video_downloads WHERE user_id = ? AND created_at >= datetime('now', '-7 days')) < ?`,
		userID, userID, RequiredReferrals,
		userID, BigVideoDailyQuota,
		userID, BigVideoWeeklyQuota)
	if err != nil {
		return 0, false, fmt.Errorf("couldn't reserve big video download: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("couldn't check big video reservation: %w", err)
	}
	if affected == 0 {
		return 0, false, nil
	}
	reservationID, err = res.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("couldn't get big video reservation id: %w", err)
	}
	return reservationID, true, nil
}

func ReleaseBigVideoDownload(reservationID int64) {
	if _, err := database.DB.Exec("DELETE FROM big_video_downloads WHERE rowid = ?", reservationID); err != nil {
		slog.Error("Couldn't release big video reservation", "ReservationID", reservationID, "Error", err.Error())
	}
}
