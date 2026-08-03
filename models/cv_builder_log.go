package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	CVBuilderActionNextStep      = "next_step"
	CVBuilderActionDownloadPDF   = "download_pdf"
	CVBuilderActionDownloadWord  = "download_word"
	CVBuilderStatusClicked       = "clicked"
	CVBuilderStatusSuccess       = "success"
	CVBuilderStatusFailed        = "failed"
)

type CVBuilderLog struct {
	ID        int64     `json:"id"`
	UserID    int       `json:"user_id"`
	Email     string    `json:"email,omitempty"`
	Username  string    `json:"username,omitempty"`
	Action    string    `json:"action"`
	FromStep  *int      `json:"from_step,omitempty"`
	ToStep    *int      `json:"to_step,omitempty"`
	FromLabel string    `json:"from_label,omitempty"`
	ToLabel   string    `json:"to_label,omitempty"`
	Format    string    `json:"format,omitempty"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CVBuilderLogFilter struct {
	Email  string
	Action string
	UserID int
	Page   int
	Limit  int
}

func CreateCVBuilderLog(db *sql.DB, entry *CVBuilderLog) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	if entry == nil {
		return fmt.Errorf("cv builder log entry is nil")
	}
	if entry.UserID <= 0 {
		return fmt.Errorf("user_id is required")
	}

	entry.Action = strings.TrimSpace(strings.ToLower(entry.Action))
	entry.Status = strings.TrimSpace(strings.ToLower(entry.Status))
	entry.Format = strings.TrimSpace(strings.ToLower(entry.Format))
	entry.FromLabel = strings.TrimSpace(entry.FromLabel)
	entry.ToLabel = strings.TrimSpace(entry.ToLabel)
	entry.Detail = strings.TrimSpace(entry.Detail)
	entry.Email = strings.TrimSpace(strings.ToLower(entry.Email))
	entry.Username = strings.TrimSpace(entry.Username)

	if entry.Action == "" {
		return fmt.Errorf("action is required")
	}
	if entry.Status == "" {
		entry.Status = CVBuilderStatusClicked
	}
	if len(entry.Detail) > 500 {
		entry.Detail = entry.Detail[:500]
	}
	if len(entry.UserAgent) > 500 {
		entry.UserAgent = entry.UserAgent[:500]
	}

	_, err := db.Exec(`
		INSERT INTO cv_builder_logs
			(user_id, email, username, action, from_step, to_step, from_label, to_label, format, status, detail, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.UserID,
		nullIfEmpty(entry.Email),
		nullIfEmpty(entry.Username),
		entry.Action,
		nullableInt(entry.FromStep),
		nullableInt(entry.ToStep),
		nullIfEmpty(entry.FromLabel),
		nullIfEmpty(entry.ToLabel),
		nullIfEmpty(entry.Format),
		entry.Status,
		nullIfEmpty(entry.Detail),
		nullIfEmpty(entry.IPAddress),
		nullIfEmpty(entry.UserAgent),
	)
	return err
}

func ListCVBuilderLogs(db *sql.DB, filter CVBuilderLogFilter) ([]CVBuilderLog, int, error) {
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
	if action := strings.TrimSpace(strings.ToLower(filter.Action)); action != "" {
		where = append(where, "action = ?")
		args = append(args, action)
	}
	if filter.UserID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, filter.UserID)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM cv_builder_logs WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, COALESCE(email, ''), COALESCE(username, ''),
		       action, from_step, to_step, COALESCE(from_label, ''), COALESCE(to_label, ''),
		       COALESCE(format, ''), status, COALESCE(detail, ''),
		       COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at
		FROM cv_builder_logs
		WHERE ` + whereSQL + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	listArgs := append(append([]interface{}{}, args...), filter.Limit, (filter.Page-1)*filter.Limit)
	rows, err := db.Query(query, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]CVBuilderLog, 0)
	for rows.Next() {
		var entry CVBuilderLog
		var fromStep, toStep sql.NullInt64
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.Email,
			&entry.Username,
			&entry.Action,
			&fromStep,
			&toStep,
			&entry.FromLabel,
			&entry.ToLabel,
			&entry.Format,
			&entry.Status,
			&entry.Detail,
			&entry.IPAddress,
			&entry.UserAgent,
			&entry.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if fromStep.Valid {
			v := int(fromStep.Int64)
			entry.FromStep = &v
		}
		if toStep.Valid {
			v := int(toStep.Int64)
			entry.ToStep = &v
		}
		logs = append(logs, entry)
	}

	return logs, total, rows.Err()
}

func nullableInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}
