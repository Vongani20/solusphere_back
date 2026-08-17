package models

import (
	"database/sql"
	"strings"
)

// --- MISSING USER STRUCT ADDED HERE ---
// User represents a user record

// -------------------------------------

// UserFace represents a user's face record
type UserFace struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	ImageURL string `json:"image_url"`
	FaceID   string `json:"face_id"`
	Status   bool   `json:"status"`
}

func (face *UserFace) IsRegistered() bool {
	return face != nil && face.Status && strings.TrimSpace(face.FaceID) != ""
}

// GetUserFaceByUserID fetches the face record for a given user
func GetUserFaceByUserID(db *sql.DB, userID int) (*UserFace, error) {
	query := "SELECT id, user_id, COALESCE(image_url, ''), COALESCE(face_id, ''), status FROM user_faces WHERE user_id = ?"
	row := db.QueryRow(query, userID)

	face := &UserFace{}
	err := row.Scan(&face.ID, &face.UserID, &face.ImageURL, &face.FaceID, &face.Status)

	if err == sql.ErrNoRows {
		// Return nil explicitly if no record is found
		return nil, nil
	}
	if err != nil {
		// Return any other actual database error
		return nil, err
	}

	return face, nil
}

func GetUserFaceRegistrationStatus(db *sql.DB, userID int) (bool, string, error) {
	face, err := GetUserFaceByUserID(db, userID)
	if err != nil {
		return false, "", err
	}
	if !face.IsRegistered() {
		return false, "", nil
	}

	return true, face.ImageURL, nil
}

// GetUserByID retrieves a user by their ID
func GetUserByID(db *sql.DB, userID int) (*User, error) {
	row := db.QueryRow("SELECT "+userSelectColumns+" FROM users WHERE id = ?", userID)
	user, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}
