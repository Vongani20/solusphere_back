package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"solusphere_backend/database"
	"solusphere_backend/internal/ai"
	"strings"
	"time"
)

const HelpdeskStatusOpen = "open"
const (
	HelpdeskStatusInProgress = "in_progress"
	HelpdeskStatusResolved   = "resolved"
	HelpdeskStatusClosed     = "closed"
)

type HelpDeskTicket struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	AIAnalysis  string    `json:"aiAnalysis"`
	AISolution  string    `json:"aiSolution"`
	AIApproach  string    `json:"aiApproach"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateTicket inserts a new ticket with AI analysis
func CreateTicket(userID int, subject, description string) (*HelpDeskTicket, error) {
	aiAnalysis := generateHelpdeskAI("Analyze ticket: "+description, "AI analysis unavailable")
	aiSolution := generateHelpdeskAI("Suggest solution for: "+description, "AI solution unavailable")
	aiApproach := generateHelpdeskAI("Step-by-step approach for: "+description, "AI approach unavailable")

	query := `INSERT INTO help_desk_tickets (user_id, subject, description, ai_analysis, ai_solution, ai_approach, status)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	res, err := database.DB.Exec(query, userID, subject, description, aiAnalysis, aiSolution, aiApproach, HelpdeskStatusOpen)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()

	return &HelpDeskTicket{
		ID:          int(id),
		UserID:      userID,
		Subject:     subject,
		Description: description,
		AIAnalysis:  aiAnalysis,
		AISolution:  aiSolution,
		AIApproach:  aiApproach,
		Status:      HelpdeskStatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func generateHelpdeskAI(prompt, fallback string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt:    "You are an experienced BPO helpdesk agent. Respond politely, professionally, and with practical operational detail.",
		UserPrompt:      prompt,
		MaxOutputTokens: 500,
		Temperature:     0.4,
	})
	if err != nil {
		fmt.Println("OpenAI helpdesk AI error:", err)
		return fallback
	}

	return result
}

func ListHelpDeskTickets() ([]HelpDeskTicket, error) {
	rows, err := database.DB.Query(`
		SELECT id, user_id, subject, description, COALESCE(ai_analysis, ''), COALESCE(ai_solution, ''),
		       COALESCE(ai_approach, ''), status, created_at, updated_at
		FROM help_desk_tickets
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]HelpDeskTicket, 0)
	for rows.Next() {
		var ticket HelpDeskTicket
		if err := rows.Scan(&ticket.ID, &ticket.UserID, &ticket.Subject, &ticket.Description, &ticket.AIAnalysis, &ticket.AISolution, &ticket.AIApproach, &ticket.Status, &ticket.CreatedAt, &ticket.UpdatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func GetHelpDeskTicketByID(ticketID int) (*HelpDeskTicket, error) {
	row := database.DB.QueryRow(`
		SELECT id, user_id, subject, description, COALESCE(ai_analysis, ''), COALESCE(ai_solution, ''),
		       COALESCE(ai_approach, ''), status, created_at, updated_at
		FROM help_desk_tickets
		WHERE id = ?
	`, ticketID)

	ticket := &HelpDeskTicket{}
	if err := row.Scan(&ticket.ID, &ticket.UserID, &ticket.Subject, &ticket.Description, &ticket.AIAnalysis, &ticket.AISolution, &ticket.AIApproach, &ticket.Status, &ticket.CreatedAt, &ticket.UpdatedAt); err != nil {
		return nil, err
	}
	return ticket, nil
}

func UpdateHelpDeskTicket(ticketID int, subject, description, status string) (*HelpDeskTicket, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "" && status != HelpdeskStatusOpen && status != HelpdeskStatusInProgress && status != HelpdeskStatusResolved && status != HelpdeskStatusClosed {
		return nil, errors.New("status must be open, in_progress, resolved, or closed")
	}
	if _, err := GetHelpDeskTicketByID(ticketID); err != nil {
		return nil, err
	}

	_, err := database.DB.Exec(`
		UPDATE help_desk_tickets
		SET subject = COALESCE(NULLIF(?, ''), subject),
		    description = COALESCE(NULLIF(?, ''), description),
		    status = COALESCE(NULLIF(?, ''), status),
		    updated_at = NOW()
		WHERE id = ?
	`, strings.TrimSpace(subject), strings.TrimSpace(description), status, ticketID)
	if err != nil {
		return nil, err
	}
	return GetHelpDeskTicketByID(ticketID)
}

func DeleteHelpDeskTicket(ticketID int) error {
	result, err := database.DB.Exec("DELETE FROM help_desk_tickets WHERE id = ?", ticketID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
