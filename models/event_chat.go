package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type Event struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	AdminUserID int       `json:"admin_user_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type EventParticipant struct {
	ID       int       `json:"id"`
	EventID  int       `json:"event_id"`
	UserID   int       `json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

type EventChatMessage struct {
	ID             int       `json:"id"`
	EventID        int       `json:"event_id"`
	SenderID       int       `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	SenderEmail    string    `json:"sender_email"`
	SenderRole     string    `json:"sender_role"`
	Sender         ChatUser  `json:"sender"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
}

func GetUserRole(db *sql.DB, userID int) (string, error) {
	var role string
	err := db.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

func IsAdmin(db *sql.DB, userID int) (bool, error) {
	role, err := GetUserRole(db, userID)
	if err != nil {
		return false, err
	}
	return role == RoleAdmin, nil
}

func CountAdmins(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	return count, err
}

func SetUserRole(db *sql.DB, userID int, role string) error {
	role = strings.TrimSpace(strings.ToLower(role))
	if role != RoleUser && role != RoleAdmin {
		return errors.New("role must be user or admin")
	}

	result, err := db.Exec("UPDATE users SET role = ? WHERE id = ?", role, userID)
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

	return nil
}

func CreateEvent(db *sql.DB, title, description, imageURL string, adminUserID int) (*Event, error) {
	result, err := db.Exec(`
		INSERT INTO events (title, description, image_url, admin_user_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'active', NOW(), NOW())
	`, title, description, imageURL, adminUserID)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	if err := JoinEvent(db, int(id), adminUserID); err != nil {
		return nil, err
	}

	return GetEventByID(db, int(id))
}

func GetEventByID(db *sql.DB, eventID int) (*Event, error) {
	row := db.QueryRow(`
		SELECT id, title, COALESCE(description, ''), COALESCE(image_url, ''), admin_user_id, status, created_at, updated_at
		FROM events
		WHERE id = ?
	`, eventID)

	event := &Event{}
	if err := row.Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.ImageURL,
		&event.AdminUserID,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return event, nil
}

func ListEvents(db *sql.DB, userID int, isAdmin bool) ([]Event, error) {
	query := `
		SELECT id, title, COALESCE(description, ''), COALESCE(image_url, ''), admin_user_id, status, created_at, updated_at
		FROM events
		WHERE status = 'active'
		ORDER BY created_at DESC
	`
	if isAdmin {
		query = `
			SELECT id, title, COALESCE(description, ''), COALESCE(image_url, ''), admin_user_id, status, created_at, updated_at
			FROM events
			ORDER BY created_at DESC
		`
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		if err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Description,
			&event.ImageURL,
			&event.AdminUserID,
			&event.Status,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

func UpdateEvent(db *sql.DB, eventID int, title, description, imageURL, status string) (*Event, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "" && status != "active" && status != "closed" {
		return nil, errors.New("status must be active or closed")
	}
	if _, err := GetEventByID(db, eventID); err != nil {
		return nil, err
	}

	_, err := db.Exec(`
		UPDATE events
		SET title = COALESCE(NULLIF(?, ''), title),
		    description = ?,
		    image_url = ?,
		    status = COALESCE(NULLIF(?, ''), status),
		    updated_at = NOW()
		WHERE id = ?
	`, strings.TrimSpace(title), strings.TrimSpace(description), strings.TrimSpace(imageURL), status, eventID)
	if err != nil {
		return nil, err
	}
	return GetEventByID(db, eventID)
}

func DeleteEvent(db *sql.DB, eventID int) error {
	result, err := db.Exec("DELETE FROM events WHERE id = ?", eventID)
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
	return nil
}

func JoinEvent(db *sql.DB, eventID, userID int) error {
	_, err := db.Exec(`
		INSERT IGNORE INTO event_participants (event_id, user_id, joined_at)
		VALUES (?, ?, NOW())
	`, eventID, userID)
	return err
}

func CanAccessEventChat(db *sql.DB, eventID, userID int, isAdmin bool) (bool, error) {
	if isAdmin {
		var exists int
		if err := db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", eventID).Scan(&exists); err != nil {
			return false, err
		}
		return exists > 0, nil
	}

	var allowed int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM events e
		LEFT JOIN event_participants ep ON ep.event_id = e.id AND ep.user_id = ?
		WHERE e.id = ? AND (e.admin_user_id = ? OR ep.id IS NOT NULL)
	`, userID, eventID, userID).Scan(&allowed)
	if err != nil {
		return false, err
	}

	return allowed > 0, nil
}

func CreateEventChatMessage(db *sql.DB, eventID, senderID int, senderRole, message string) (*EventChatMessage, error) {
	result, err := db.Exec(`
		INSERT INTO event_chat_messages (event_id, sender_id, sender_role, message, created_at)
		VALUES (?, ?, ?, ?, NOW())
	`, eventID, senderID, senderRole, message)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetEventChatMessageByID(db, int(id))
}

func GetEventChatMessageByID(db *sql.DB, messageID int) (*EventChatMessage, error) {
	row := db.QueryRow(`
		SELECT m.id, m.event_id, m.sender_id, u.username, u.email, m.sender_role, m.message, m.created_at
		FROM event_chat_messages m
		INNER JOIN users u ON u.id = m.sender_id
		WHERE m.id = ?
	`, messageID)

	message := &EventChatMessage{}
	if err := row.Scan(
		&message.ID,
		&message.EventID,
		&message.SenderID,
		&message.SenderUsername,
		&message.SenderEmail,
		&message.SenderRole,
		&message.Message,
		&message.CreatedAt,
	); err != nil {
		return nil, err
	}
	message.Sender = ChatUser{
		ID:       message.SenderID,
		Username: message.SenderUsername,
		Email:    message.SenderEmail,
		Role:     message.SenderRole,
	}

	return message, nil
}

func ListEventChatMessages(db *sql.DB, eventID, limit int) ([]EventChatMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := db.Query(`
		SELECT m.id, m.event_id, m.sender_id, u.username, u.email, m.sender_role, m.message, m.created_at
		FROM event_chat_messages m
		INNER JOIN users u ON u.id = m.sender_id
		WHERE m.event_id = ?
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT ?
	`, eventID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]EventChatMessage, 0)
	for rows.Next() {
		var message EventChatMessage
		if err := rows.Scan(
			&message.ID,
			&message.EventID,
			&message.SenderID,
			&message.SenderUsername,
			&message.SenderEmail,
			&message.SenderRole,
			&message.Message,
			&message.CreatedAt,
		); err != nil {
			return nil, err
		}
		message.Sender = ChatUser{
			ID:       message.SenderID,
			Username: message.SenderUsername,
			Email:    message.SenderEmail,
			Role:     message.SenderRole,
		}
		messages = append(messages, message)
	}

	return messages, rows.Err()
}
