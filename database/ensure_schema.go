package database

import (
	"database/sql"
	"log"
	"strings"
)

// EnsureChatAndCallSchema creates chat attachment and call signaling tables if migrations did not run.
func EnsureChatAndCallSchema(db *sql.DB) error {
	if db == nil {
		return nil
	}

	steps := []struct {
		name string
		fn   func(*sql.DB) error
	}{
		{name: "direct message attachments", fn: ensureDirectMessageAttachmentColumns},
		{name: "call sessions", fn: ensureCallSessionTables},
	}

	for _, step := range steps {
		if err := step.fn(db); err != nil {
			return err
		}
		log.Printf("Schema ensure OK: %s", step.name)
	}
	return nil
}

func ensureDirectMessageAttachmentColumns(db *sql.DB) error {
	if !tableExists(db, "direct_messages") {
		return nil
	}

	if columnExists(db, "direct_messages", "message_type") {
		return nil
	}

	_, err := db.Exec(`
		ALTER TABLE direct_messages
			MODIFY message TEXT NULL,
			ADD COLUMN message_type VARCHAR(20) NOT NULL DEFAULT 'text' AFTER message,
			ADD COLUMN attachment_url TEXT NULL AFTER message_type,
			ADD COLUMN attachment_mime VARCHAR(100) NULL AFTER attachment_url
	`)
	return err
}

func ensureCallSessionTables(db *sql.DB) error {
	if tableExists(db, "call_sessions") && tableExists(db, "call_ice_candidates") {
		return nil
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS call_sessions (
			id CHAR(36) PRIMARY KEY,
			caller_id INT NOT NULL,
			callee_id INT NOT NULL,
			call_type VARCHAR(10) NOT NULL DEFAULT 'audio',
			status VARCHAR(20) NOT NULL DEFAULT 'ringing',
			offer_sdp MEDIUMTEXT NULL,
			answer_sdp MEDIUMTEXT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			ended_at TIMESTAMP NULL,
			FOREIGN KEY (caller_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (callee_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_call_sessions_callee_status (callee_id, status),
			INDEX idx_call_sessions_caller_status (caller_id, status)
		)
	`); err != nil {
		return err
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS call_ice_candidates (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			call_id CHAR(36) NOT NULL,
			sender_id INT NOT NULL,
			candidate TEXT NOT NULL,
			sdp_mid VARCHAR(64) NULL,
			sdp_m_line_index INT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (call_id) REFERENCES call_sessions(id) ON DELETE CASCADE,
			INDEX idx_call_ice_call_id (call_id, id)
		)
	`)
	return err
}

func tableExists(db *sql.DB, table string) bool {
	var name sql.NullString
	err := db.QueryRow(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
		LIMIT 1
	`, table).Scan(&name)
	return err == nil && name.Valid
}

func columnExists(db *sql.DB, table, column string) bool {
	var name sql.NullString
	err := db.QueryRow(`
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?
		LIMIT 1
	`, table, column).Scan(&name)
	return err == nil && name.Valid
}

func IsMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "does not exist")
}
