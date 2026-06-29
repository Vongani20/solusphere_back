package models

import (
	"database/sql"
	"time"
)

const PresenceOnlineWindow = 2 * time.Minute

type UserPresence struct {
	UserID     int        `json:"user_id"`
	IsOnline   bool       `json:"is_online"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

func TouchUserPresence(db *sql.DB, userID int) error {
	_, err := db.Exec(`
		INSERT INTO user_presence (user_id, last_seen_at)
		VALUES (?, NOW())
		ON DUPLICATE KEY UPDATE last_seen_at = NOW()
	`, userID)
	return err
}

func presenceSelectExpr(alias string) string {
	return `
		CASE
			WHEN ` + alias + `.last_seen_at IS NOT NULL
			     AND ` + alias + `.last_seen_at >= (NOW() - INTERVAL 2 MINUTE)
			THEN 1
			ELSE 0
		END AS is_online,
		` + alias + `.last_seen_at
	`
}

func scanPresenceFields(isOnline *bool, lastSeenAt **time.Time, isOnlineRaw int, lastSeen sql.NullTime) {
	*isOnline = isOnlineRaw == 1
	if lastSeen.Valid {
		value := lastSeen.Time
		*lastSeenAt = &value
	} else {
		*lastSeenAt = nil
	}
}

func ListUserPresence(db *sql.DB, currentUserID int) ([]UserPresence, error) {
	rows, err := db.Query(`
		SELECT u.id,
		       `+presenceSelectExpr("p")+`
		FROM users u
		LEFT JOIN user_presence p ON p.user_id = u.id
		WHERE u.id <> ?
		ORDER BY is_online DESC, u.username ASC
	`, currentUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserPresence, 0)
	for rows.Next() {
		var item UserPresence
		var isOnlineRaw int
		var lastSeen sql.NullTime
		if err := rows.Scan(&item.UserID, &isOnlineRaw, &lastSeen); err != nil {
			return nil, err
		}
		scanPresenceFields(&item.IsOnline, &item.LastSeenAt, isOnlineRaw, lastSeen)
		items = append(items, item)
	}
	return items, rows.Err()
}
