package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type ChatbotRequest struct {
	Message string `json:"message" binding:"required"`
}

type ChatbotResponse struct {
	Reply string `json:"reply"`
	Error string `json:"error,omitempty"`
}

// ChatbotHandler handles chatbot messages using Gemini API
func ChatbotHandler(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ChatbotRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ChatbotResponse{
				Error: "Invalid request format",
			})
			return
		}

		if req.Message == "" {
			c.JSON(http.StatusBadRequest, ChatbotResponse{
				Error: "Message cannot be empty",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create Gemini client
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			c.JSON(http.StatusInternalServerError, ChatbotResponse{
				Error: "Failed to initialize AI service",
			})
			return
		}
		defer client.Close()

		// Use gemini-pro model
		model := client.GenerativeModel("gemini-pro")

		// Configure the model
		model.SetTemperature(0.7)
		model.SetTopK(40)
		model.SetTopP(0.95)
		model.SetMaxOutputTokens(1024)

		// Create the prompt
		prompt := "You are SIA (Smart Intelligence Assistant), a helpful and professional AI assistant. " +
			"Respond politely, professionally, and concisely to the following message:\n\n" +
			req.Message

		// Generate response
		resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			c.JSON(http.StatusInternalServerError, ChatbotResponse{
				Error: "Failed to generate response",
			})
			return
		}

		// Extract text from response
		if resp == nil || len(resp.Candidates) == 0 {
			c.JSON(http.StatusInternalServerError, ChatbotResponse{
				Error: "No response generated",
			})
			return
		}

		var reply string
		for _, part := range resp.Candidates[0].Content.Parts {
			if text, ok := part.(genai.Text); ok {
				reply += string(text)
			}
		}

		if reply == "" {
			c.JSON(http.StatusInternalServerError, ChatbotResponse{
				Error: "Empty response from AI",
			})
			return
		}

		c.JSON(http.StatusOK, ChatbotResponse{
			Reply: reply,
		})
	}
}
