package models

import (
	"fmt"
	"solusphere_backend/database"
	helpers "solusphere_backend/helper"
	"time"
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
	aiAnalysis, err := helpers.CallGeminiAI("Analyze ticket: " + description)
	if err != nil {
		fmt.Println("AI Analysis error:", err)
		aiAnalysis = "AI analysis unavailable"
	}

	aiSolution, err := helpers.CallGeminiAI("Suggest solution for: " + description)
	if err != nil {
		fmt.Println("AI Solution error:", err)
		aiSolution = "AI solution unavailable"
	}

	aiApproach, err := helpers.CallGeminiAI("Step-by-step approach for: " + description)
	if err != nil {
		fmt.Println("AI Approach error:", err)
		aiApproach = "AI approach unavailable"
	}

	query := `INSERT INTO help_desk_tickets (user_id, subject, description, ai_analysis, ai_solution, ai_approach, status)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	res, err := database.DB.Exec(query, userID, subject, description, aiAnalysis, aiSolution, aiApproach, "pending")
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
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}
