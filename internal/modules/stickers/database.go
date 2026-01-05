package stickers

import (
	"database/sql"
	"errors"

	"github.com/ruizlenato/smudgelord/internal/database"
)

const maxPacks = 10

type StickerPack struct {
	ID        int64
	UserID    int64
	PackName  string
	IsDefault bool
}

func getUserPacks(userID int64) ([]StickerPack, error) {
	rows, err := database.DB.Query(
		"SELECT id, user_id, pack_name, is_default FROM sticker_packs WHERE user_id = ? ORDER BY id",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []StickerPack
	for rows.Next() {
		var pack StickerPack
		if err := rows.Scan(&pack.ID, &pack.UserID, &pack.PackName, &pack.IsDefault); err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

func getUserPacksCount(userID int64) (int, error) {
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM sticker_packs WHERE user_id = ?", userID).Scan(&count)
	return count, err
}

func getDefaultPack(userID int64) (*StickerPack, error) {
	var pack StickerPack
	err := database.DB.QueryRow(
		"SELECT id, user_id, pack_name, is_default FROM sticker_packs WHERE user_id = ? AND is_default = 1",
		userID,
	).Scan(&pack.ID, &pack.UserID, &pack.PackName, &pack.IsDefault)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pack, nil
}

func createPack(userID int64, packName string) error {
	count, err := getUserPacksCount(userID)
	if err != nil {
		return err
	}
	if count >= maxPacks {
		return errors.New("max packs reached")
	}

	_, err = database.DB.Exec(
		"INSERT INTO sticker_packs (user_id, pack_name, is_default) VALUES (?, ?, 0)",
		userID, packName,
	)
	return err
}

func clearDefaultPack(userID int64) error {
	_, err := database.DB.Exec("UPDATE sticker_packs SET is_default = 0 WHERE user_id = ?", userID)
	return err
}

func setDefaultPack(userID int64, packID int64) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE sticker_packs SET is_default = 0 WHERE user_id = ?", userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE sticker_packs SET is_default = 1 WHERE id = ? AND user_id = ?", packID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func deletePack(userID int64, packID int64) error {
	result, err := database.DB.Exec("DELETE FROM sticker_packs WHERE id = ? AND user_id = ?", packID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("pack not found")
	}

	return nil
}

func packExists(packName string) (bool, error) {
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM sticker_packs WHERE pack_name = ?", packName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
