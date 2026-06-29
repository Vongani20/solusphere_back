package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	ConsentTypeAIDocumentProcessing = "ai_document_processing"
	ConsentVersionAIDocument        = "1.0"
)

type UserConsent struct {
	ID              int       `json:"id"`
	UserID          int       `json:"user_id"`
	ConsentType     string    `json:"consent_type"`
	ConsentVersion  string    `json:"consent_version"`
	SignedName      string    `json:"signed_name"`
	AcceptedAt      time.Time `json:"accepted_at"`
	IPAddress       string    `json:"ip_address,omitempty"`
	UserAgent       string    `json:"user_agent,omitempty"`
}

type ConsentStatus struct {
	ConsentType    string     `json:"consent_type"`
	Version        string     `json:"version"`
	Signed         bool       `json:"signed"`
	SignedName     string     `json:"signed_name,omitempty"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
}

func HasUserConsent(db *sql.DB, userID int, consentType, version string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database connection is nil")
	}

	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM user_consents
		WHERE user_id = ? AND consent_type = ? AND consent_version = ?
	`, userID, consentType, version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func RecordUserConsent(db *sql.DB, consent *UserConsent) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	if consent == nil {
		return fmt.Errorf("consent is nil")
	}

	consent.ConsentType = strings.TrimSpace(consent.ConsentType)
	consent.ConsentVersion = strings.TrimSpace(consent.ConsentVersion)
	consent.SignedName = strings.TrimSpace(consent.SignedName)
	if consent.ConsentType == "" || consent.ConsentVersion == "" || consent.SignedName == "" {
		return fmt.Errorf("consent type, version, and signed name are required")
	}

	_, err := db.Exec(`
		INSERT INTO user_consents (user_id, consent_type, consent_version, signed_name, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			signed_name = VALUES(signed_name),
			accepted_at = CURRENT_TIMESTAMP,
			ip_address = VALUES(ip_address),
			user_agent = VALUES(user_agent)
	`, consent.UserID, consent.ConsentType, consent.ConsentVersion, consent.SignedName, consent.IPAddress, consent.UserAgent)
	return err
}

func ListUserConsentStatus(db *sql.DB, userID int) ([]ConsentStatus, error) {
	required := []struct {
		consentType string
		version     string
	}{
		{ConsentTypeAIDocumentProcessing, ConsentVersionAIDocument},
	}

	statuses := make([]ConsentStatus, 0, len(required))
	for _, item := range required {
		status := ConsentStatus{
			ConsentType: item.consentType,
			Version:     item.version,
			Signed:      false,
		}

		var signedName, ipAddress, userAgent sql.NullString
		var acceptedAt sql.NullTime
		err := db.QueryRow(`
			SELECT signed_name, accepted_at, ip_address, user_agent
			FROM user_consents
			WHERE user_id = ? AND consent_type = ? AND consent_version = ?
		`, userID, item.consentType, item.version).Scan(&signedName, &acceptedAt, &ipAddress, &userAgent)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil {
			status.Signed = true
			if signedName.Valid {
				status.SignedName = signedName.String
			}
			if acceptedAt.Valid {
				t := acceptedAt.Time
				status.AcceptedAt = &t
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}
