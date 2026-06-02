package models

import (
	"database/sql"
)

// --- MISSING USER STRUCT ADDED HERE ---
// User represents a user record

// -------------------------------------

// UserFace represents a user's face record
type UserFace struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	ImageURL string `json:"image_url"`
	Status   bool   `json:"status"`
}

// GetUserFaceByUserID fetches the face record for a given user
func GetUserFaceByUserID(db *sql.DB, userID int) (*UserFace, error) {
	query := "SELECT id, user_id, image_url, status FROM user_faces WHERE user_id = ?"
	row := db.QueryRow(query, userID)

	face := &UserFace{}
	err := row.Scan(&face.ID, &face.UserID, &face.ImageURL, &face.Status)

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

// GetUserByID retrieves a user by their ID
func GetUserByID(db *sql.DB, userID int) (*User, error) {
	query := "SELECT id, username, email, password, created_at FROM users WHERE id = ?"
	row := db.QueryRow(query, userID)

	user := &User{}
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt)

	if err == sql.ErrNoRows {
		// Return nil explicitly if no record is found
		return nil, nil
	}
	if err != nil {
		// Return any other actual database error
		return nil, err
	}

	return user, nil
}
