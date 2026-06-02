package models

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// HashPassword hashes the user's password
func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword compares the provided password with the hashed password
func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

// CreateUser inserts a new user into the database

// GetUserByUsername retrieves a user by username
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	query := "SELECT id, username, email, password, created_at FROM users WHERE username = ?"
	row := db.QueryRow(query, username)

	user := &User{}
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	query := "SELECT id, username, email, password, created_at FROM users WHERE email = ?"
	row := db.QueryRow(query, email)

	user := &User{}
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// CreateUser inserts a new user into the database
func CreateUser(db *sql.DB, user *User) error {
	log.Printf("🔧 CreateUser called")
	log.Printf("   DB pointer: %p", db)

	if db == nil {
		log.Printf("❌ CreateUser: db is nil!")
		return fmt.Errorf("database connection is nil")
	}

	// Check if database is closed
	if err := db.Ping(); err != nil {
		log.Printf("❌ CreateUser: Ping failed: %v", err)
		return fmt.Errorf("database ping failed: %v", err)
	}

	log.Printf("✅ CreateUser: Database connection is healthy")

	query := "INSERT INTO users (username, email, password) VALUES (?, ?, ?)"
	log.Printf("Executing query: %s", query)

	result, err := db.Exec(query, user.Username, user.Email, user.Password)
	if err != nil {
		log.Printf("❌ CreateUser: Exec error: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("❌ CreateUser: LastInsertId error: %v", err)
		return err
	}
	user.ID = int(id)
	log.Printf("✅ CreateUser: Success! User ID: %d", user.ID)
	return nil
}
