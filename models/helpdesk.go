package models

import (
	"context"
	"fmt"
	"solusphere_backend/database"
	"solusphere_backend/internal/ai"
	"time"
)

const HelpdeskStatusOpen = "open"

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
