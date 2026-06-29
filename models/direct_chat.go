package models

import (
	"database/sql"
	"strings"
	"time"
)

const (
	DirectMessageTypeText  = "text"
	DirectMessageTypeImage = "image"
	DirectMessageTypeVoice = "voice"
)

type ChatUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type DirectMessage struct {
	ID               int        `json:"id"`
	SenderID         int        `json:"sender_id"`
	SenderUsername   string     `json:"sender_username"`
	ReceiverID       int        `json:"receiver_id"`
	ReceiverUsername string     `json:"receiver_username"`
	Message          string     `json:"message"`
	MessageType      string     `json:"message_type"`
	AttachmentURL    string     `json:"attachment_url,omitempty"`
	AttachmentMIME   string     `json:"attachment_mime,omitempty"`
	ReadAt           *time.Time `json:"read_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

type DirectConversation struct {
	UserID          int       `json:"user_id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	Role            string    `json:"role"`
	LatestMessage   string    `json:"latest_message"`
	LatestMessageAt time.Time `json:"latest_message_at"`
	UnreadCount     int       `json:"unread_count"`
}

func directMessageSelectColumns() string {
	return `
		m.id, m.sender_id, sender.username, m.receiver_id, receiver.username,
		COALESCE(m.message, ''), m.message_type, COALESCE(m.attachment_url, ''),
		COALESCE(m.attachment_mime, ''), m.read_at, m.created_at
	`
}

func scanDirectMessage(scanner interface {
	Scan(dest ...any) error
}) (*DirectMessage, error) {
	message := &DirectMessage{}
	if err := scanner.Scan(
		&message.ID,
		&message.SenderID,
		&message.SenderUsername,
		&message.ReceiverID,
		&message.ReceiverUsername,
		&message.Message,
		&message.MessageType,
		&message.AttachmentURL,
		&message.AttachmentMIME,
		&message.ReadAt,
		&message.CreatedAt,
	); err != nil {
		return nil, err
	}
	presignDirectMessageAttachment(message)
	return message, nil
}

func presignDirectMessageAttachment(message *DirectMessage) {
	if message == nil {
		return
	}
	if strings.TrimSpace(message.AttachmentURL) != "" {
		message.AttachmentURL = ClientAccessiblePhotoURL(message.AttachmentURL)
	}
}

func ListChatUsers(db *sql.DB, currentUserID int) ([]ChatUser, error) {
	rows, err := db.Query(`
		SELECT id, username, email, role
		FROM users
		WHERE id <> ?
		ORDER BY username ASC
	`, currentUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]ChatUser, 0)
	for rows.Next() {
		var user ChatUser
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func CreateDirectMessage(
	db *sql.DB,
	senderID, receiverID int,
	messageType, message, attachmentURL, attachmentMIME string,
) (*DirectMessage, error) {
	messageType = strings.TrimSpace(messageType)
	if messageType == "" {
		messageType = DirectMessageTypeText
	}

	result, err := db.Exec(`
		INSERT INTO direct_messages (sender_id, receiver_id, message, message_type, attachment_url, attachment_mime, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW())
	`, senderID, receiverID, nullIfEmpty(message), messageType, nullIfEmpty(attachmentURL), nullIfEmpty(attachmentMIME))
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetDirectMessageByID(db, int(id))
}

func GetDirectMessageByID(db *sql.DB, messageID int) (*DirectMessage, error) {
	row := db.QueryRow(`
		SELECT `+directMessageSelectColumns()+`
		FROM direct_messages m
		INNER JOIN users sender ON sender.id = m.sender_id
		INNER JOIN users receiver ON receiver.id = m.receiver_id
		WHERE m.id = ?
	`, messageID)

	return scanDirectMessage(row)
}

func ListDirectMessages(db *sql.DB, currentUserID, otherUserID, limit int) ([]DirectMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := db.Query(`
		SELECT `+directMessageSelectColumns()+`
		FROM direct_messages m
		INNER JOIN users sender ON sender.id = m.sender_id
		INNER JOIN users receiver ON receiver.id = m.receiver_id
		WHERE (m.sender_id = ? AND m.receiver_id = ?)
		   OR (m.sender_id = ? AND m.receiver_id = ?)
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT ?
	`, currentUserID, otherUserID, otherUserID, currentUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]DirectMessage, 0)
	for rows.Next() {
		message, err := scanDirectMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, MarkDirectMessagesRead(db, currentUserID, otherUserID)
}

func MarkDirectMessagesRead(db *sql.DB, currentUserID, otherUserID int) error {
	_, err := db.Exec(`
		UPDATE direct_messages
		SET read_at = NOW()
		WHERE receiver_id = ? AND sender_id = ? AND read_at IS NULL
	`, currentUserID, otherUserID)
	return err
}

func conversationPreviewExpr() string {
	return `
		CASE
		    WHEN latest.message_type = 'image' THEN COALESCE(NULLIF(latest.message, ''), 'Photo')
		    WHEN latest.message_type = 'voice' THEN COALESCE(NULLIF(latest.message, ''), 'Voice note')
		    ELSE COALESCE(latest.message, '')
		END
	`
}

func ListDirectConversations(db *sql.DB, currentUserID int) ([]DirectConversation, error) {
	rows, err := db.Query(`
		SELECT other_user.id, other_user.username, other_user.email, other_user.role,
		       `+conversationPreviewExpr()+`, latest.created_at,
		       (
		           SELECT COUNT(*)
		           FROM direct_messages unread
		           WHERE unread.sender_id = other_user.id
		             AND unread.receiver_id = ?
		             AND unread.read_at IS NULL
		       ) AS unread_count
		FROM (
		    SELECT
		        CASE
		            WHEN sender_id = ? THEN receiver_id
		            ELSE sender_id
		        END AS other_user_id,
		        MAX(id) AS latest_message_id
		    FROM direct_messages
		    WHERE sender_id = ? OR receiver_id = ?
		    GROUP BY other_user_id
		) convo
		INNER JOIN direct_messages latest ON latest.id = convo.latest_message_id
		INNER JOIN users other_user ON other_user.id = convo.other_user_id
		ORDER BY latest.created_at DESC, latest.id DESC
	`, currentUserID, currentUserID, currentUserID, currentUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := make([]DirectConversation, 0)
	for rows.Next() {
		var conversation DirectConversation
		if err := rows.Scan(
			&conversation.UserID,
			&conversation.Username,
			&conversation.Email,
			&conversation.Role,
			&conversation.LatestMessage,
			&conversation.LatestMessageAt,
			&conversation.UnreadCount,
		); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	return conversations, rows.Err()
}
