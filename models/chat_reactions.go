package models

import (
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MessageReaction is one user's emoji reaction to a direct message.
type MessageReaction struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Emoji    string `json:"emoji"`
}

var ErrInvalidReaction = errors.New("reaction must be a single emoji")

// ValidReactionEmoji accepts short emoji strings (including ZWJ sequences and skin tones)
// and rejects plain text.
func ValidReactionEmoji(emoji string) bool {
	if emoji == "" || len(emoji) > 32 || !utf8.ValidString(emoji) || utf8.RuneCountInString(emoji) > 10 {
		return false
	}
	hasSymbol := false
	for _, r := range emoji {
		switch {
		case r == '\u200d' || r == '\ufe0f' || (r >= 0x1F3FB && r <= 0x1F3FF):
			// zero-width joiner, variation selector, skin tones
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || unicode.IsControl(r) || r < 0x80:
			return false
		default:
			hasSymbol = true
		}
	}
	return hasSymbol
}

// ToggleMessageReaction sets the user's reaction; reacting again with the same emoji removes it.
// Only the sender or receiver of the message may react. Returns the message's reactions.
func ToggleMessageReaction(db *sql.DB, messageID, userID int, emoji string) ([]MessageReaction, error) {
	emoji = strings.TrimSpace(emoji)
	if !ValidReactionEmoji(emoji) {
		return nil, ErrInvalidReaction
	}

	var senderID, receiverID int
	if err := db.QueryRow(`SELECT sender_id, receiver_id FROM direct_messages WHERE id = ?`, messageID).
		Scan(&senderID, &receiverID); err != nil {
		return nil, err
	}
	if userID != senderID && userID != receiverID {
		return nil, sql.ErrNoRows
	}

	var current sql.NullString
	err := db.QueryRow(`SELECT emoji FROM direct_message_reactions WHERE message_id = ? AND user_id = ?`, messageID, userID).
		Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if current.Valid && current.String == emoji {
		_, err = db.Exec(`DELETE FROM direct_message_reactions WHERE message_id = ? AND user_id = ?`, messageID, userID)
	} else {
		_, err = db.Exec(`
			INSERT INTO direct_message_reactions (message_id, user_id, emoji, created_at)
			VALUES (?, ?, ?, NOW())
			ON DUPLICATE KEY UPDATE emoji = VALUES(emoji), created_at = NOW()
		`, messageID, userID, emoji)
	}
	if err != nil {
		return nil, err
	}

	messages := []DirectMessage{{ID: messageID}}
	if err := attachMessageReactions(db, messages); err != nil {
		return nil, err
	}
	return messages[0].Reactions, nil
}

func attachMessageReactions(db *sql.DB, messages []DirectMessage) error {
	if len(messages) == 0 {
		return nil
	}
	index := make(map[int]int, len(messages))
	placeholders := make([]string, 0, len(messages))
	args := make([]interface{}, 0, len(messages))
	for i := range messages {
		messages[i].Reactions = []MessageReaction{}
		index[messages[i].ID] = i
		placeholders = append(placeholders, "?")
		args = append(args, messages[i].ID)
	}

	rows, err := db.Query(`
		SELECT r.message_id, r.user_id, u.username, r.emoji
		FROM direct_message_reactions r
		INNER JOIN users u ON u.id = r.user_id
		WHERE r.message_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY r.created_at ASC
	`, args...)
	if err != nil {
		if strings.Contains(err.Error(), "doesn't exist") {
			return nil // table not created yet; reactions are optional
		}
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var messageID int
		var reaction MessageReaction
		if err := rows.Scan(&messageID, &reaction.UserID, &reaction.Username, &reaction.Emoji); err != nil {
			return err
		}
		if i, ok := index[messageID]; ok {
			messages[i].Reactions = append(messages[i].Reactions, reaction)
		}
	}
	return rows.Err()
}
