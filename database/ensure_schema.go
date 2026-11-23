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
		{name: "user presence", fn: ensureUserPresenceTable},
		{name: "innovation ideas", fn: ensureInnovationIdeasTable},
		{name: "cv builder logs", fn: ensureCVBuilderLogsTable},
		{name: "rfp documents", fn: ensureRFPDocumentsTable},
		{name: "chat emoji charset", fn: ensureDirectMessagesUTF8MB4},
		{name: "chat reactions and group calls", fn: ensureChatReactionsAndGroupCallTables},
		{name: "user birth dates", fn: ensureUserBirthDateColumn},
		{name: "typing, profile photos and tasks", fn: ensureTypingPhotosAndTasks},
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
		return ensureCallCalleeSeenColumn(db)
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
	if err != nil {
		return err
	}
	return ensureCallCalleeSeenColumn(db)
}

func ensureCallCalleeSeenColumn(db *sql.DB) error {
	if !tableExists(db, "call_sessions") || columnExists(db, "call_sessions", "callee_seen_at") {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE call_sessions ADD COLUMN callee_seen_at TIMESTAMP NULL AFTER ended_at`)
	return err
}

func ensureUserPresenceTable(db *sql.DB) error {
	if tableExists(db, "user_presence") {
		return nil
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_presence (
			user_id INT PRIMARY KEY,
			last_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_user_presence_last_seen (last_seen_at)
		)
	`)
	return err
}

func ensureInnovationIdeasTable(db *sql.DB) error {
	if tableExists(db, "innovation_ideas") {
		return nil
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS innovation_ideas (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			full_name VARCHAR(200) NOT NULL,
			department VARCHAR(200) NOT NULL,
			email VARCHAR(255) NOT NULL,
			title VARCHAR(300) NOT NULL,
			description TEXT NOT NULL,
			problem TEXT NOT NULL,
			solution TEXT NULL,
			photo_url TEXT NULL,
			declaration_accepted TINYINT(1) NOT NULL DEFAULT 0,
			signature VARCHAR(255) NULL,
			reviewer VARCHAR(200) NULL,
			status VARCHAR(40) NOT NULL DEFAULT 'submitted',
			comments TEXT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_innovation_ideas_user_id (user_id),
			INDEX idx_innovation_ideas_status (status),
			INDEX idx_innovation_ideas_created_at (created_at)
		)
	`)
	return err
}

func ensureCVBuilderLogsTable(db *sql.DB) error {
	if tableExists(db, "cv_builder_logs") {
		return nil
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cv_builder_logs (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			email VARCHAR(255) NULL,
			username VARCHAR(255) NULL,
			action VARCHAR(40) NOT NULL,
			from_step INT NULL,
			to_step INT NULL,
			from_label VARCHAR(100) NULL,
			to_label VARCHAR(100) NULL,
			format VARCHAR(20) NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'clicked',
			detail VARCHAR(500) NULL,
			ip_address VARCHAR(64) NULL,
			user_agent VARCHAR(500) NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_cv_builder_logs_user_id (user_id),
			INDEX idx_cv_builder_logs_action (action),
			INDEX idx_cv_builder_logs_created_at (created_at)
		)
	`)
	return err
}

func ensureRFPDocumentsTable(db *sql.DB) error {
	if tableExists(db, "rfp_documents") {
		return nil
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS rfp_documents (
			id CHAR(36) PRIMARY KEY,
			user_id INT NOT NULL,
			original_filename VARCHAR(1024) NOT NULL DEFAULT '',
			sources_json LONGTEXT NULL,
			extracted_text LONGTEXT NULL,
			analysis_source VARCHAR(20) NOT NULL DEFAULT 'builtin',
			model VARCHAR(100) NULL,
			truncated TINYINT(1) NOT NULL DEFAULT 0,
			warning TEXT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'processing',
			template_json LONGTEXT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_rfp_documents_user_created (user_id, created_at)
		)
	`)
	return err
}

func ensureUserBirthDateColumn(db *sql.DB) error {
	if !tableExists(db, "users") || columnExists(db, "users", "birth_date") {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE users ADD COLUMN birth_date DATE NULL AFTER role`)
	return err
}

// ensureDirectMessagesUTF8MB4 makes sure emoji (4-byte UTF-8) can be stored in chat messages.
func ensureDirectMessagesUTF8MB4(db *sql.DB) error {
	if !tableExists(db, "direct_messages") {
		return nil
	}
	var collation sql.NullString
	if err := db.QueryRow(`
		SELECT table_collation FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = 'direct_messages'
	`).Scan(&collation); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(collation.String), "utf8mb4") {
		return nil
	}
	log.Printf("Converting direct_messages from %s to utf8mb4 for emoji support", collation.String)
	_, err := db.Exec(`ALTER TABLE direct_messages CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`)
	return err
}

func ensureChatReactionsAndGroupCallTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS direct_message_reactions (
    message_id INT NOT NULL,
    user_id INT NOT NULL,
    emoji VARCHAR(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (message_id, user_id),
    FOREIGN KEY (message_id) REFERENCES direct_messages(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) DEFAULT CHARSET = utf8mb4`,
		`CREATE TABLE IF NOT EXISTS group_calls (
    id CHAR(36) PRIMARY KEY,
    host_id INT NOT NULL,
    call_type VARCHAR(10) NOT NULL DEFAULT 'audio',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP NULL,
    FOREIGN KEY (host_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_group_calls_status (status, created_at)
)`,
		`CREATE TABLE IF NOT EXISTS group_call_participants (
    call_id CHAR(36) NOT NULL,
    user_id INT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'invited',
    invited_by INT NULL,
    invited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    joined_at TIMESTAMP NULL,
    left_at TIMESTAMP NULL,
    last_seen_at TIMESTAMP NULL,
    PRIMARY KEY (call_id, user_id),
    FOREIGN KEY (call_id) REFERENCES group_calls(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_group_call_participants_user (user_id, status)
)`,
		`CREATE TABLE IF NOT EXISTS group_call_signals (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    call_id CHAR(36) NOT NULL,
    from_user_id INT NOT NULL,
    to_user_id INT NOT NULL,
    kind VARCHAR(20) NOT NULL,
    payload MEDIUMTEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (call_id) REFERENCES group_calls(id) ON DELETE CASCADE,
    INDEX idx_group_call_signals_to (call_id, to_user_id, id)
)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureTypingPhotosAndTasks backs migration 000029 when migrations did not run.
func ensureTypingPhotosAndTasks(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_typing (
			user_id INT NOT NULL,
			peer_id INT NOT NULL,
			typed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, peer_id),
			INDEX idx_chat_typing_peer (peer_id, user_id)
		)`); err != nil {
		return err
	}
	if tableExists(db, "users") && !columnExists(db, "users", "profile_photo_url") {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN profile_photo_url TEXT NULL`); err != nil {
			return err
		}
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id INT AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(200) NOT NULL,
			description TEXT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'todo',
			priority VARCHAR(10) NOT NULL DEFAULT 'medium',
			due_date DATE NULL,
			created_by INT NOT NULL,
			assigned_to INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			completed_at TIMESTAMP NULL,
			FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (assigned_to) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_tasks_assigned_status (assigned_to, status),
			INDEX idx_tasks_created_by (created_by, status)
		) DEFAULT CHARSET = utf8mb4`)
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
