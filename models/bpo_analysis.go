package models

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type BPOAnalysis struct {
	ID              string    `json:"id"`
	Filename        string    `json:"filename"`
	FilePath        string    `json:"file_path"`
	FileSize        int64     `json:"file_size"`
	MimeType        string    `json:"mime_type"`
	ExtractedText   string    `json:"extracted_text"`
	AnalysisResult  string    `json:"analysis_result"` // Store as JSON string
	Status          string    `json:"status"`
	PageCount       int       `json:"page_count"`
	AnalysisType    string    `json:"analysis_type"`
	ConfidenceScore float64   `json:"confidence_score"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Analysis status constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Analysis type constants
const (
	TypeInvoice  = "invoice"
	TypeContract = "contract"
	TypeReport   = "report"
	TypeForm     = "form"
	TypeGeneral  = "general"
)

// CreateBPOAnalysis inserts a new analysis record
func CreateBPOAnalysis(db *sql.DB, analysis *BPOAnalysis) error {
	query := `INSERT INTO bpo_analyses 
        (id, filename, file_path, file_size, mime_type, status, created_at, updated_at) 
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query,
		analysis.ID,
		analysis.Filename,
		analysis.FilePath,
		analysis.FileSize,
		analysis.MimeType,
		analysis.Status,
		analysis.CreatedAt,
		analysis.UpdatedAt,
	)
	return err
}

// GetBPOAnalysisByID retrieves an analysis by ID
func GetBPOAnalysisByID(db *sql.DB, id string) (*BPOAnalysis, error) {
	query := `SELECT id, filename, file_path, file_size, mime_type, extracted_text, 
                     analysis_result, status, page_count, analysis_type, confidence_score, 
                     created_at, updated_at 
              FROM bpo_analyses WHERE id = ?`

	row := db.QueryRow(query, id)

	analysis := &BPOAnalysis{}
	var analysisResult sql.NullString
	var extractedText sql.NullString
	var updatedAt sql.NullTime

	err := row.Scan(
		&analysis.ID,
		&analysis.Filename,
		&analysis.FilePath,
		&analysis.FileSize,
		&analysis.MimeType,
		&extractedText,
		&analysisResult,
		&analysis.Status,
		&analysis.PageCount,
		&analysis.AnalysisType,
		&analysis.ConfidenceScore,
		&analysis.CreatedAt,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if extractedText.Valid {
		analysis.ExtractedText = extractedText.String
	}
	if analysisResult.Valid {
		analysis.AnalysisResult = analysisResult.String
	}
	if updatedAt.Valid {
		analysis.UpdatedAt = updatedAt.Time
	}

	return analysis, nil
}

// UpdateBPOAnalysis updates an analysis record
func UpdateBPOAnalysis(db *sql.DB, analysis *BPOAnalysis) error {
	query := `UPDATE bpo_analyses SET 
        filename = ?, file_path = ?, file_size = ?, mime_type = ?, 
        extracted_text = ?, analysis_result = ?, status = ?, 
        page_count = ?, analysis_type = ?, confidence_score = ?, updated_at = ?
        WHERE id = ?`

	_, err := db.Exec(query,
		analysis.Filename,
		analysis.FilePath,
		analysis.FileSize,
		analysis.MimeType,
		nullIfEmpty(analysis.ExtractedText),
		nullJSONIfEmpty(analysis.AnalysisResult),
		analysis.Status,
		analysis.PageCount,
		analysis.AnalysisType,
		analysis.ConfidenceScore,
		analysis.UpdatedAt,
		analysis.ID,
	)
	return err
}

func nullIfEmpty(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullJSONIfEmpty(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// ListBPOAnalysesByStatuses returns analyses in the given statuses (newest first).
func ListBPOAnalysesByStatuses(db *sql.DB, statuses []string) ([]*BPOAnalysis, error) {
	if len(statuses) == 0 {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(statuses))
	placeholders = strings.TrimSuffix(placeholders, ",")

	query := `SELECT id, filename, file_path, file_size, mime_type, extracted_text,
                     analysis_result, status, page_count, analysis_type, confidence_score,
                     created_at, updated_at
              FROM bpo_analyses
              WHERE status IN (` + placeholders + `)
              ORDER BY created_at ASC`

	args := make([]interface{}, len(statuses))
	for i, status := range statuses {
		args[i] = status
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBPOAnalysisRows(rows)
}

func scanBPOAnalysisRows(rows *sql.Rows) ([]*BPOAnalysis, error) {
	var analyses []*BPOAnalysis
	for rows.Next() {
		analysis := &BPOAnalysis{}
		var analysisResult sql.NullString
		var extractedText sql.NullString
		var updatedAt sql.NullTime

		err := rows.Scan(
			&analysis.ID,
			&analysis.Filename,
			&analysis.FilePath,
			&analysis.FileSize,
			&analysis.MimeType,
			&extractedText,
			&analysisResult,
			&analysis.Status,
			&analysis.PageCount,
			&analysis.AnalysisType,
			&analysis.ConfidenceScore,
			&analysis.CreatedAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if extractedText.Valid {
			analysis.ExtractedText = extractedText.String
		}
		if analysisResult.Valid {
			analysis.AnalysisResult = analysisResult.String
		}
		if updatedAt.Valid {
			analysis.UpdatedAt = updatedAt.Time
		}

		analyses = append(analyses, analysis)
	}

	return analyses, rows.Err()
}

// ListBPOAnalyses retrieves paginated analyses with optional filters
func ListBPOAnalyses(db *sql.DB, page, limit int, status, analysisType string) ([]*BPOAnalysis, int, error) {
	// Build count query
	countQuery := "SELECT COUNT(*) FROM bpo_analyses WHERE 1=1"
	countArgs := []interface{}{}

	if status != "" {
		countQuery += " AND status = ?"
		countArgs = append(countArgs, status)
	}
	if analysisType != "" {
		countQuery += " AND analysis_type = ?"
		countArgs = append(countArgs, analysisType)
	}

	var total int
	err := db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Build main query
	query := `SELECT id, filename, file_path, file_size, mime_type, extracted_text, 
                     analysis_result, status, page_count, analysis_type, confidence_score, 
                     created_at, updated_at 
              FROM bpo_analyses WHERE 1=1`
	args := []interface{}{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if analysisType != "" {
		query += " AND analysis_type = ?"
		args = append(args, analysisType)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, (page-1)*limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	analyses, err := scanBPOAnalysisRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return analyses, total, nil
}

// DeleteBPOAnalysis deletes an analysis record
func DeleteBPOAnalysis(db *sql.DB, id string) error {
	query := "DELETE FROM bpo_analyses WHERE id = ?"
	_, err := db.Exec(query, id)
	return err
}

// Convert map to JSON string for storage
func AnalysisResultToJSON(result map[string]interface{}) (string, error) {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// Convert JSON string back to map
func JSONToAnalysisResult(jsonStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
