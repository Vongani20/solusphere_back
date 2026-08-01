package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	InnovationStatusSubmitted = "submitted"
	InnovationStatusInReview  = "in_review"
	InnovationStatusApproved  = "approved"
	InnovationStatusRejected  = "rejected"
)

type InnovationIdea struct {
	ID                   int        `json:"id"`
	UserID               int        `json:"user_id"`
	Username             string     `json:"username,omitempty"`
	FullName             string     `json:"full_name"`
	Department           string     `json:"department"`
	Email                string     `json:"email"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	Problem              string     `json:"problem"`
	Solution             string     `json:"solution,omitempty"`
	PhotoURL             string     `json:"photo_url,omitempty"`
	DeclarationAccepted  bool       `json:"declaration_accepted"`
	Signature            string     `json:"signature,omitempty"`
	Reviewer             string     `json:"reviewer,omitempty"`
	Status               string     `json:"status"`
	Comments             string     `json:"comments,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func NormalizeInnovationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case InnovationStatusInReview, "in review":
		return InnovationStatusInReview
	case InnovationStatusApproved:
		return InnovationStatusApproved
	case InnovationStatusRejected:
		return InnovationStatusRejected
	default:
		return InnovationStatusSubmitted
	}
}

func CreateInnovationIdea(db *sql.DB, idea *InnovationIdea) (*InnovationIdea, error) {
	if idea == nil {
		return nil, errors.New("idea is required")
	}
	idea.FullName = strings.TrimSpace(idea.FullName)
	idea.Department = strings.TrimSpace(idea.Department)
	idea.Email = strings.TrimSpace(idea.Email)
	idea.Title = strings.TrimSpace(idea.Title)
	idea.Description = strings.TrimSpace(idea.Description)
	idea.Problem = strings.TrimSpace(idea.Problem)
	idea.Solution = strings.TrimSpace(idea.Solution)
	idea.Signature = strings.TrimSpace(idea.Signature)
	idea.Status = NormalizeInnovationStatus(idea.Status)
	if idea.Status == "" {
		idea.Status = InnovationStatusSubmitted
	}

	if idea.FullName == "" || idea.Department == "" || idea.Email == "" || idea.Title == "" || idea.Description == "" || idea.Problem == "" {
		return nil, errors.New("required innovation fields are missing")
	}
	if !idea.DeclarationAccepted {
		return nil, errors.New("declaration must be accepted")
	}

	result, err := db.Exec(`
		INSERT INTO innovation_ideas (
			user_id, full_name, department, email, title, description, problem, solution,
			photo_url, declaration_accepted, signature, reviewer, status, comments, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`,
		idea.UserID,
		idea.FullName,
		idea.Department,
		idea.Email,
		idea.Title,
		idea.Description,
		idea.Problem,
		nullIfEmptyString(idea.Solution),
		nullIfEmptyString(idea.PhotoURL),
		idea.DeclarationAccepted,
		nullIfEmptyString(idea.Signature),
		nullIfEmptyString(idea.Reviewer),
		idea.Status,
		nullIfEmptyString(idea.Comments),
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetInnovationIdeaByID(db, int(id))
}

func GetInnovationIdeaByID(db *sql.DB, id int) (*InnovationIdea, error) {
	row := db.QueryRow(`
		SELECT i.id, i.user_id, COALESCE(u.username, ''), i.full_name, i.department, i.email,
		       i.title, i.description, i.problem, COALESCE(i.solution, ''), COALESCE(i.photo_url, ''),
		       i.declaration_accepted, COALESCE(i.signature, ''), COALESCE(i.reviewer, ''),
		       i.status, COALESCE(i.comments, ''), i.created_at, i.updated_at
		FROM innovation_ideas i
		LEFT JOIN users u ON u.id = i.user_id
		WHERE i.id = ?
	`, id)
	return scanInnovationIdea(row)
}

func ListInnovationIdeasByUser(db *sql.DB, userID int) ([]InnovationIdea, error) {
	rows, err := db.Query(`
		SELECT i.id, i.user_id, COALESCE(u.username, ''), i.full_name, i.department, i.email,
		       i.title, i.description, i.problem, COALESCE(i.solution, ''), COALESCE(i.photo_url, ''),
		       i.declaration_accepted, COALESCE(i.signature, ''), COALESCE(i.reviewer, ''),
		       i.status, COALESCE(i.comments, ''), i.created_at, i.updated_at
		FROM innovation_ideas i
		LEFT JOIN users u ON u.id = i.user_id
		WHERE i.user_id = ?
		ORDER BY i.created_at DESC, i.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInnovationIdeas(rows)
}

func ListAllInnovationIdeas(db *sql.DB) ([]InnovationIdea, error) {
	rows, err := db.Query(`
		SELECT i.id, i.user_id, COALESCE(u.username, ''), i.full_name, i.department, i.email,
		       i.title, i.description, i.problem, COALESCE(i.solution, ''), COALESCE(i.photo_url, ''),
		       i.declaration_accepted, COALESCE(i.signature, ''), COALESCE(i.reviewer, ''),
		       i.status, COALESCE(i.comments, ''), i.created_at, i.updated_at
		FROM innovation_ideas i
		LEFT JOIN users u ON u.id = i.user_id
		ORDER BY i.created_at DESC, i.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInnovationIdeas(rows)
}

func UpdateInnovationIdeaCommittee(db *sql.DB, id int, reviewer, status, comments string) (*InnovationIdea, error) {
	status = NormalizeInnovationStatus(status)
	_, err := db.Exec(`
		UPDATE innovation_ideas
		SET reviewer = ?, status = ?, comments = ?, updated_at = NOW()
		WHERE id = ?
	`, nullIfEmptyString(strings.TrimSpace(reviewer)), status, nullIfEmptyString(strings.TrimSpace(comments)), id)
	if err != nil {
		return nil, err
	}
	return GetInnovationIdeaByID(db, id)
}

func scanInnovationIdeas(rows *sql.Rows) ([]InnovationIdea, error) {
	items := make([]InnovationIdea, 0)
	for rows.Next() {
		idea, err := scanInnovationIdea(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *idea)
	}
	return items, rows.Err()
}

func scanInnovationIdea(scanner interface {
	Scan(dest ...any) error
}) (*InnovationIdea, error) {
	idea := &InnovationIdea{}
	var accepted int
	if err := scanner.Scan(
		&idea.ID,
		&idea.UserID,
		&idea.Username,
		&idea.FullName,
		&idea.Department,
		&idea.Email,
		&idea.Title,
		&idea.Description,
		&idea.Problem,
		&idea.Solution,
		&idea.PhotoURL,
		&accepted,
		&idea.Signature,
		&idea.Reviewer,
		&idea.Status,
		&idea.Comments,
		&idea.CreatedAt,
		&idea.UpdatedAt,
	); err != nil {
		return nil, err
	}
	idea.DeclarationAccepted = accepted == 1
	if idea.PhotoURL != "" {
		idea.PhotoURL = ClientAccessiblePhotoURL(idea.PhotoURL)
	}
	return idea, nil
}
