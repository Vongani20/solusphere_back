package models

import (
	"database/sql"
	"encoding/json"
	"strings"
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
	ProfilePhotoURL    string              `json:"profile_photo_url,omitempty"`
	ProfessionalSkills []ProfessionalSkill `json:"professional_skills"`
	Qualifications     []string            `json:"qualifications"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// CVSearchOptions controls talent directory filtering.
type CVSearchOptions struct {
	Skill         string
	Qualification string
	Query         string
	Match         string // "all" (default) or "any"
}

// CVSearchMatchAny combines skill and qualification groups with OR instead of AND.
const CVSearchMatchAny = "any"

// SanitizeCVProfile trims fields and removes empty list entries before validation/persistence.
func SanitizeCVProfile(p *CVProfile) {
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)
	p.ProfileText = strings.TrimSpace(p.ProfileText)
	p.ValueProposition = strings.TrimSpace(p.ValueProposition)
	p.Gender = strings.TrimSpace(p.Gender)
	p.Nationality = strings.TrimSpace(p.Nationality)
	p.DateOfBirth = NormalizeDateOfBirth(p.DateOfBirth)

	p.Qualifications = filterNonEmptyStrings(p.Qualifications)
	p.ComputerSkills = filterNonEmptyStrings(p.ComputerSkills)
	p.ProfessionalMemberships = filterNonEmptyStrings(p.ProfessionalMemberships)
	p.Languages = filterNonEmptyStrings(p.Languages)

	var skills []ProfessionalSkill
	for _, s := range p.ProfessionalSkills {
		s.Skill = strings.TrimSpace(s.Skill)
		s.Details = filterNonEmptyStrings(s.Details)
		if s.Skill != "" || len(s.Details) > 0 {
			skills = append(skills, s)
		}
	}
	p.ProfessionalSkills = skills

	var experience []CVExperience
	for _, e := range p.Experience {
		e.Company = strings.TrimSpace(e.Company)
		e.Position = strings.TrimSpace(e.Position)
		e.PeriodStart = strings.TrimSpace(e.PeriodStart)
		e.PeriodEnd = strings.TrimSpace(e.PeriodEnd)
		e.ScopeOfWork = filterNonEmptyStrings(e.ScopeOfWork)
		if e.Company != "" || e.Position != "" || e.PeriodStart != "" || e.PeriodEnd != "" || len(e.ScopeOfWork) > 0 {
			experience = append(experience, e)
		}
	}
	p.Experience = experience
}

// NormalizeDateOfBirth converts common frontend/API formats to YYYY-MM-DD for MySQL DATE.
func NormalizeDateOfBirth(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) >= 10 && raw[4] == '-' && raw[7] == '-' {
		return raw[:10]
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02T15:04:05", raw); err == nil {
		return t.Format("2006-01-02")
	}
	return raw
}

func filterNonEmptyStrings(values []string) []string {
	var out []string
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func UpsertCVProfile(db *sql.DB, profile *CVProfile) error {
	SanitizeCVProfile(profile)

	skillsJSON, _ := json.Marshal(profile.ProfessionalSkills)
	qualsJSON, _ := json.Marshal(profile.Qualifications)
	compJSON, _ := json.Marshal(profile.ComputerSkills)
	memJSON, _ := json.Marshal(profile.ProfessionalMemberships)
	langsJSON, _ := json.Marshal(profile.Languages)
	expJSON, _ := json.Marshal(profile.Experience)

	var dob interface{}
	if profile.DateOfBirth == "" {
		dob = nil
	} else {
		dob = profile.DateOfBirth
	}

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
		dob,
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

// ListCVProfiles returns CV summaries with optional advanced text filters.
func ListCVProfiles(db *sql.DB, opts CVSearchOptions) ([]CVProfileSummary, error) {
	query := `SELECT user_id, first_name, last_name, profile_photo_url, professional_skills, qualifications, updated_at
	          FROM cv_profiles
	          WHERE TRIM(COALESCE(first_name, '')) <> ''
	            AND TRIM(COALESCE(last_name, '')) <> ''`
	args := []interface{}{}

	skillTerms := splitSearchTerms(opts.Skill)
	qualTerms := splitSearchTerms(opts.Qualification)
	queryTerms := splitSearchTerms(opts.Query)

	skillClause, skillArgs := buildCVTermGroupClause(skillTerms)
	qualClause, qualArgs := buildCVTermGroupClause(qualTerms)
	queryClause, queryArgs := buildCVTermGroupClause(queryTerms)

	groupClauses := make([]string, 0, 3)
	groupArgs := make([]interface{}, 0)

	if skillClause != "" {
		groupClauses = append(groupClauses, skillClause)
		groupArgs = append(groupArgs, skillArgs...)
	}
	if qualClause != "" {
		groupClauses = append(groupClauses, qualClause)
		groupArgs = append(groupArgs, qualArgs...)
	}
	if queryClause != "" {
		groupClauses = append(groupClauses, queryClause)
		groupArgs = append(groupArgs, queryArgs...)
	}

	if len(groupClauses) > 0 {
		joiner := " AND "
		if strings.EqualFold(strings.TrimSpace(opts.Match), CVSearchMatchAny) {
			joiner = " OR "
		}
		query += " AND (" + strings.Join(groupClauses, joiner) + ")"
		args = append(args, groupArgs...)
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
		var photoURL sql.NullString
		if err := rows.Scan(&s.UserID, &s.FirstName, &s.LastName, &photoURL, &skillsJSON, &qualsJSON, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if photoURL.Valid {
			s.ProfilePhotoURL = ClientAccessiblePhotoURL(photoURL.String)
		}
		unmarshalJSON(skillsJSON, &s.ProfessionalSkills)
		unmarshalJSON(qualsJSON, &s.Qualifications)
		summaries = append(summaries, s)
	}
	return summaries, nil
}

func splitSearchTerms(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '|'
	})
	terms := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		term := strings.TrimSpace(part)
		if term == "" {
			continue
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}

func cvSearchableColumns() []string {
	return []string{
		"first_name",
		"last_name",
		"profile_text",
		"value_proposition",
		"professional_skills",
		"qualifications",
		"computer_skills",
		"languages",
		"experience",
	}
}

func buildCVTermGroupClause(terms []string) (string, []interface{}) {
	if len(terms) == 0 {
		return "", nil
	}

	columns := cvSearchableColumns()
	termClauses := make([]string, 0, len(terms))
	args := make([]interface{}, 0, len(terms)*len(columns))

	for _, term := range terms {
		pattern := "%" + term + "%"
		columnClauses := make([]string, 0, len(columns))
		for _, column := range columns {
			columnClauses = append(columnClauses, column+" LIKE ?")
			args = append(args, pattern)
		}
		termClauses = append(termClauses, "("+strings.Join(columnClauses, " OR ")+")")
	}

	return "(" + strings.Join(termClauses, " AND ") + ")", args
}

func DeleteCVProfileByUserID(db *sql.DB, userID int) (bool, error) {
	result, err := db.Exec(`DELETE FROM cv_profiles WHERE user_id = ?`, userID)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func unmarshalJSON(ns sql.NullString, dst interface{}) {
	if ns.Valid && ns.String != "" && ns.String != "null" {
		json.Unmarshal([]byte(ns.String), dst)
	}
}
