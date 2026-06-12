package models

import (
	"database/sql"
	"time"
)

func CreatePasswordResetCode(db *sql.DB, userID int, codeHash string, expiresAt time.Time) error {
	_, err := db.Exec(
		"INSERT INTO password_reset_codes (user_id, code_hash, expires_at) VALUES (?, ?, ?)",
		userID,
		codeHash,
		expiresAt,
	)
	return err
}

func GetValidPasswordResetCodeID(db *sql.DB, userID int, codeHash string, now time.Time) (int, error) {
	var id int
	err := db.QueryRow(
		`SELECT id
		FROM password_reset_codes
		WHERE user_id = ? AND code_hash = ? AND used_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1`,
		userID,
		codeHash,
		now,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func MarkPasswordResetCodeUsed(db *sql.DB, resetCodeID int) error {
	_, err := db.Exec("UPDATE password_reset_codes SET used_at = CURRENT_TIMESTAMP WHERE id = ?", resetCodeID)
	return err
}

func ExpireUserPasswordResetCodes(db *sql.DB, userID int) error {
	_, err := db.Exec("UPDATE password_reset_codes SET used_at = CURRENT_TIMESTAMP WHERE user_id = ? AND used_at IS NULL", userID)
	return err
}
