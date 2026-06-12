package models

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PhoneNumber  string    `json:"phone_number,omitempty"`
	Password     string    `json:"-"`
	AuthProvider string    `json:"auth_provider"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password" binding:"required,min=6"`
}

func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	query := "SELECT id, username, email, COALESCE(phone_number, ''), password, COALESCE(auth_provider, 'local'), role, created_at FROM users WHERE username = ?"
	row := db.QueryRow(query, username)

	user := &User{}
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PhoneNumber, &user.Password, &user.AuthProvider, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	query := "SELECT id, username, email, COALESCE(phone_number, ''), password, COALESCE(auth_provider, 'local'), role, created_at FROM users WHERE email = ?"
	row := db.QueryRow(query, email)

	user := &User{}
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PhoneNumber, &user.Password, &user.AuthProvider, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func CreateUser(db *sql.DB, user *User) error {
	log.Printf("CreateUser called")
	log.Printf("   DB pointer: %p", db)

	if db == nil {
		log.Printf("CreateUser: db is nil")
		return fmt.Errorf("database connection is nil")
	}

	if err := db.Ping(); err != nil {
		log.Printf("CreateUser: Ping failed: %v", err)
		return fmt.Errorf("database ping failed: %v", err)
	}

	if user.AuthProvider == "" {
		user.AuthProvider = "local"
	}

	query := "INSERT INTO users (username, email, phone_number, password, auth_provider) VALUES (?, ?, ?, ?, ?)"
	log.Printf("Executing query: %s", query)

	result, err := db.Exec(query, user.Username, user.Email, user.PhoneNumber, user.Password, user.AuthProvider)
	if err != nil {
		log.Printf("CreateUser: Exec error: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("CreateUser: LastInsertId error: %v", err)
		return err
	}
	user.ID = int(id)
	log.Printf("CreateUser: Success. User ID: %d", user.ID)
	return nil
}

func UpdateUserPassword(db *sql.DB, userID int, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE users SET password = ? WHERE id = ?", string(hashedPassword), userID)
	return err
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`
		SELECT id, username, email, COALESCE(phone_number, ''), password, COALESCE(auth_provider, 'local'), role, created_at
		FROM users
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.PhoneNumber, &user.Password, &user.AuthProvider, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func UpdateUserProfile(db *sql.DB, userID int, username, email, phoneNumber, role string) (*User, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	if role != "" && role != RoleUser && role != RoleAdmin {
		return nil, fmt.Errorf("role must be user or admin")
	}
	if _, err := GetUserByID(db, userID); err != nil {
		return nil, err
	}

	_, err := db.Exec(`
		UPDATE users
		SET username = COALESCE(NULLIF(?, ''), username),
		    email = COALESCE(NULLIF(?, ''), email),
		    phone_number = ?,
		    role = COALESCE(NULLIF(?, ''), role)
		WHERE id = ?
	`, strings.TrimSpace(username), strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(phoneNumber), role, userID)
	if err != nil {
		return nil, err
	}
	return GetUserByID(db, userID)
}

func DeleteUser(db *sql.DB, userID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		"DELETE m FROM event_chat_messages m INNER JOIN events e ON e.id = m.event_id WHERE e.admin_user_id = ?",
		"DELETE p FROM event_participants p INNER JOIN events e ON e.id = p.event_id WHERE e.admin_user_id = ?",
		"DELETE FROM events WHERE admin_user_id = ?",
		"DELETE FROM event_chat_messages WHERE sender_id = ?",
		"DELETE FROM event_participants WHERE user_id = ?",
		"DELETE FROM direct_messages WHERE sender_id = ? OR receiver_id = ?",
		"DELETE FROM help_desk_tickets WHERE user_id = ?",
		"DELETE FROM file_uploads WHERE user_id = ?",
		"DELETE FROM user_faces WHERE user_id = ?",
		"DELETE FROM password_reset_codes WHERE user_id = ?",
	}
	for _, query := range queries {
		if strings.Contains(query, " OR receiver_id") {
			if _, err := tx.Exec(query, userID, userID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(query, userID); err != nil {
			return err
		}
	}

	result, err := tx.Exec("DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}
