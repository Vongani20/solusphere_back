package models

import "database/sql"

// TypingWindowSeconds is how long a "typing" signal stays visible after the last keystroke ping.
const TypingWindowSeconds = 6

// TouchTyping records that userID is typing to peerID.
func TouchTyping(db *sql.DB, userID, peerID int) error {
	_, err := db.Exec(`
		INSERT INTO chat_typing (user_id, peer_id, typed_at) VALUES (?, ?, NOW())
		ON DUPLICATE KEY UPDATE typed_at = NOW()`, userID, peerID)
	return err
}

// ClearTyping removes userID's typing state toward peerID (for example after sending).
func ClearTyping(db *sql.DB, userID, peerID int) error {
	_, err := db.Exec(`DELETE FROM chat_typing WHERE user_id = ? AND peer_id = ?`, userID, peerID)
	return err
}

// IsTyping reports whether peerID typed to userID within the typing window.
func IsTyping(db *sql.DB, peerID, userID int) (bool, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM chat_typing
		WHERE user_id = ? AND peer_id = ? AND typed_at >= NOW() - INTERVAL ? SECOND`,
		peerID, userID, TypingWindowSeconds).Scan(&n)
	return n > 0, err
}
