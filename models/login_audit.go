package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	LoginMethodPassword = "password"
	LoginMethodFace     = "face"
	LoginStatusSuccess  = "success"
	LoginStatusFailed   = "failed"
)

type LoginAuditLog struct {
	ID            int64     `json:"id"`
	UserID        *int      `json:"user_id,omitempty"`
	Email         string    `json:"email,omitempty"`
	Username      string    `json:"username,omitempty"`
	LoginMethod   string    `json:"login_method"`
	Status        string    `json:"status"`
	FailureReason string    `json:"failure_reason,omitempty"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type LoginAuditFilter struct {
	Email   string
	Status  string
	Method  string
	UserID  int
	Page    int
	Limit   int
}

func CreateLoginAuditLog(db *sql.DB, entry *LoginAuditLog) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	if entry == nil {
		return fmt.Errorf("login audit entry is nil")
	}

	entry.LoginMethod = strings.TrimSpace(entry.LoginMethod)
	entry.Status = strings.TrimSpace(entry.Status)
	if entry.LoginMethod == "" || entry.Status == "" {
		return fmt.Errorf("login method and status are required")
	}

	if len(entry.FailureReason) > 255 {
		entry.FailureReason = entry.FailureReason[:255]
	}
	if len(entry.UserAgent) > 500 {
		entry.UserAgent = entry.UserAgent[:500]
	}

	var userID interface{}
	if entry.UserID != nil && *entry.UserID > 0 {
		userID = *entry.UserID
	}

	_, err := db.Exec(`
		INSERT INTO login_audit_logs
			(user_id, email, username, login_method, status, failure_reason, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		userID,
		nullIfEmpty(entry.Email),
		nullIfEmpty(entry.Username),
		entry.LoginMethod,
		entry.Status,
		nullIfEmpty(entry.FailureReason),
		nullIfEmpty(entry.IPAddress),
		nullIfEmpty(entry.UserAgent),
	)
	return err
}

func ListLoginAuditLogs(db *sql.DB, filter LoginAuditFilter) ([]LoginAuditLog, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 200 {
		filter.Limit = 50
	}

	where := []string{"1=1"}
	args := []interface{}{}

	if email := strings.TrimSpace(strings.ToLower(filter.Email)); email != "" {
		where = append(where, "LOWER(email) LIKE ?")
		args = append(args, "%"+email+"%")
	}
	if status := strings.TrimSpace(strings.ToLower(filter.Status)); status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if method := strings.TrimSpace(strings.ToLower(filter.Method)); method != "" {
		where = append(where, "login_method = ?")
		args = append(args, method)
	}
	if filter.UserID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, filter.UserID)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM login_audit_logs WHERE " + whereSQL
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, COALESCE(email, ''), COALESCE(username, ''),
		       login_method, status, COALESCE(failure_reason, ''),
		       COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at
		FROM login_audit_logs
		WHERE ` + whereSQL + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	listArgs := append(append([]interface{}{}, args...), filter.Limit, (filter.Page-1)*filter.Limit)
	rows, err := db.Query(query, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]LoginAuditLog, 0)
	for rows.Next() {
		var entry LoginAuditLog
		var userID sql.NullInt64
		if err := rows.Scan(
			&entry.ID,
			&userID,
			&entry.Email,
			&entry.Username,
			&entry.LoginMethod,
			&entry.Status,
			&entry.FailureReason,
			&entry.IPAddress,
			&entry.UserAgent,
			&entry.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if userID.Valid {
			id := int(userID.Int64)
			entry.UserID = &id
		}
		logs = append(logs, entry)
	}

	return logs, total, rows.Err()
}
