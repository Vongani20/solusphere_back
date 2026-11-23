package models

import (
	"database/sql"
	"strings"
)

// ProfilePhotoForClient turns a stored S3 photo URL into a short-lived signed URL the browser can load.
func ProfilePhotoForClient(stored string) string {
	if strings.TrimSpace(stored) == "" {
		return ""
	}
	return ClientAccessiblePhotoURL(stored)
}

// GetProfilePhotoURL returns the stored (unsigned) profile photo URL for a user, or "".
func GetProfilePhotoURL(db *sql.DB, userID int) (string, error) {
	var url sql.NullString
	err := db.QueryRow(`SELECT profile_photo_url FROM users WHERE id = ?`, userID).Scan(&url)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return strings.TrimSpace(url.String), err
}

// SetProfilePhotoURL stores a profile photo URL; an empty value removes the photo.
func SetProfilePhotoURL(db *sql.DB, userID int, url string) error {
	_, err := db.Exec(`UPDATE users SET profile_photo_url = ? WHERE id = ?`, nullIfEmpty(url), userID)
	return err
}
