package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ProfessionalSkill struct {
	Skill   string   `json:"skill"`
	Details []string `json:"details"`
}

type CVExperience struct {
	Company     string   `json:"company"`
	Position    string   `json:"position"`
	PeriodStart string   `json:"period_start"`
	PeriodEnd   string   `json:"period_end"`
	ScopeOfWork []string `json:"scope_of_work"`
}

type CVProfile struct {
	ID                      int                 `json:"id"`
	UserID                  int                 `json:"user_id"`
	FirstName               string              `json:"first_name"`
	LastName                string              `json:"last_name"`
	ProfilePhotoURL         string              `json:"profile_photo_url"`
	ProfileText             string              `json:"profile_text"`
	ValueProposition        string              `json:"value_proposition"`
	Gender                  string              `json:"gender"`
	Nationality             string              `json:"nationality"`
	DateOfBirth             string              `json:"date_of_birth"`
	ProfessionalSkills      []ProfessionalSkill `json:"professional_skills"`
	Qualifications          []string            `json:"qualifications"`
	ComputerSkills          []string            `json:"computer_skills"`
	ProfessionalMemberships []string            `json:"professional_memberships"`
	Languages               []string            `json:"languages"`
	Experience              []CVExperience      `json:"experience"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
}

type CVProfileSummary struct {
	UserID             int                 `json:"user_id"`
	FirstName          string              `json:"first_name"`
	LastName           string              `json:"last_name"`
	ProfessionalSkills []ProfessionalSkill `json:"professional_skills"`
	Qualifications     []string            `json:"qualifications"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

func UpsertCVProfile(db *sql.DB, profile *CVProfile) error {
	skillsJSON, _ := json.Marshal(profile.ProfessionalSkills)
	qualsJSON, _ := json.Marshal(profile.Qualifications)
	compJSON, _ := json.Marshal(profile.ComputerSkills)
	memJSON, _ := json.Marshal(profile.ProfessionalMemberships)
	langsJSON, _ := json.Marshal(profile.Languages)
	expJSON, _ := json.Marshal(profile.Experience)

	query := `
		INSERT INTO cv_profiles
		(user_id, first_name, last_name, profile_photo_url, profile_text, value_proposition,
		 gender, nationality, date_of_birth, professional_skills, qualifications,
		 computer_skills, professional_memberships, languages, experience)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  first_name               = VALUES(first_name),
		  last_name                = VALUES(last_name),
		  profile_text             = VALUES(profile_text),
		  value_proposition        = VALUES(value_proposition),
		  gender                   = VALUES(gender),
		  nationality              = VALUES(nationality),
		  date_of_birth            = VALUES(date_of_birth),
		  professional_skills      = VALUES(professional_skills),
		  qualifications           = VALUES(qualifications),
		  computer_skills          = VALUES(computer_skills),
		  professional_memberships = VALUES(professional_memberships),
		  languages                = VALUES(languages),
		  experience               = VALUES(experience),
		  updated_at               = CURRENT_TIMESTAMP
	`

	_, err := db.Exec(query,
		profile.UserID,
		profile.FirstName,
		profile.LastName,
		profile.ProfilePhotoURL,
		profile.ProfileText,
		profile.ValueProposition,
		profile.Gender,
		profile.Nationality,
		profile.DateOfBirth,
		string(skillsJSON),
		string(qualsJSON),
		string(compJSON),
		string(memJSON),
		string(langsJSON),
		string(expJSON),
	)
	return err
}

func GetCVProfileByUserID(db *sql.DB, userID int) (*CVProfile, error) {
	query := `
		SELECT id, user_id, first_name, last_name, profile_photo_url, profile_text,
		       value_proposition, gender, nationality, date_of_birth,
		       professional_skills, qualifications, computer_skills,
		       professional_memberships, languages, experience, created_at, updated_at
		FROM cv_profiles WHERE user_id = ?
	`

	row := db.QueryRow(query, userID)
	p := &CVProfile{}

	var (
		photoURL    sql.NullString
		gender      sql.NullString
		nationality sql.NullString
		dob         sql.NullString
		skills      sql.NullString
		quals       sql.NullString
		comp        sql.NullString
		mem         sql.NullString
		langs       sql.NullString
		exp         sql.NullString
	)

	err := row.Scan(
		&p.ID, &p.UserID, &p.FirstName, &p.LastName,
		&photoURL, &p.ProfileText, &p.ValueProposition,
		&gender, &nationality, &dob,
		&skills, &quals, &comp, &mem, &langs, &exp,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if photoURL.Valid {
		p.ProfilePhotoURL = photoURL.String
	}
	if gender.Valid {
		p.Gender = gender.String
	}
	if nationality.Valid {
		p.Nationality = nationality.String
	}
	if dob.Valid {
		p.DateOfBirth = dob.String
	}

	unmarshalJSON(skills, &p.ProfessionalSkills)
	unmarshalJSON(quals, &p.Qualifications)
	unmarshalJSON(comp, &p.ComputerSkills)
	unmarshalJSON(mem, &p.ProfessionalMemberships)
	unmarshalJSON(langs, &p.Languages)
	unmarshalJSON(exp, &p.Experience)

	return p, nil
}

func UpdateCVProfilePhotoURL(db *sql.DB, userID int, photoURL string) error {
	_, err := db.Exec(
		`UPDATE cv_profiles SET profile_photo_url = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`,
		photoURL, userID,
	)
	return err
}

// UpsertCVProfilePhotoURL inserts a minimal cv_profiles row for userID if one does not
// yet exist, then sets profile_photo_url in either case.  This avoids passing empty
// strings for DATE/NOT-NULL columns when the user uploads a photo before filling in
// the rest of their CV.
func UpsertCVProfilePhotoURL(db *sql.DB, userID int, photoURL string) error {
	_, err := db.Exec(`
		INSERT INTO cv_profiles (user_id, profile_photo_url)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
		  profile_photo_url = VALUES(profile_photo_url),
		  updated_at        = CURRENT_TIMESTAMP
	`, userID, photoURL)
	return err
}

// ListCVProfiles returns CV summaries with optional substring filters.
// skill filters on professional_skills JSON; qualification filters on qualifications JSON.
func ListCVProfiles(db *sql.DB, skill, qualification string) ([]CVProfileSummary, error) {
	query := `SELECT user_id, first_name, last_name, professional_skills, qualifications, updated_at
	          FROM cv_profiles WHERE 1=1`
	args := []interface{}{}

	if skill != "" {
		query += " AND professional_skills LIKE ?"
		args = append(args, "%"+skill+"%")
	}
	if qualification != "" {
		query += " AND qualifications LIKE ?"
		args = append(args, "%"+qualification+"%")
	}
	query += " ORDER BY updated_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []CVProfileSummary
	for rows.Next() {
		var s CVProfileSummary
		var skillsJSON, qualsJSON sql.NullString
		if err := rows.Scan(&s.UserID, &s.FirstName, &s.LastName, &skillsJSON, &qualsJSON, &s.UpdatedAt); err != nil {
			return nil, err
		}
		unmarshalJSON(skillsJSON, &s.ProfessionalSkills)
		unmarshalJSON(qualsJSON, &s.Qualifications)
		summaries = append(summaries, s)
	}
	return summaries, nil
}

func unmarshalJSON(ns sql.NullString, dst interface{}) {
	if ns.Valid && ns.String != "" && ns.String != "null" {
		json.Unmarshal([]byte(ns.String), dst)
	}
}
