package handlers

import (
	"net/http"
	"strings"

	"solusphere_backend/internal/ai"
	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

type ChatbotRequest struct {
	Message   string `json:"message" binding:"required"`
	WebSearch *bool  `json:"web_search,omitempty"`
}

type ChatbotResponse struct {
	Reply            string        `json:"reply"`
	Sources          []ai.Citation `json:"sources,omitempty"`
	SourceCount      int           `json:"source_count"`
	Model            string        `json:"model,omitempty"`
	WebSearchEnabled bool          `json:"web_search_enabled"`
	Error            string        `json:"error,omitempty"`
}

// ChatbotHandler handles chatbot messages using OpenAI.
func ChatbotHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !services.IsOpenAIInitialized() {
			c.JSON(http.StatusServiceUnavailable, ChatbotResponse{
				Error: "OpenAI is not configured. Set OPENAI_API_KEY to enable chatbot responses.",
			})
			return
		}

		var req ChatbotRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ChatbotResponse{
				Error: "Invalid request format",
			})
			return
		}

		req.Message = strings.TrimSpace(req.Message)
		if req.Message == "" {
			c.JSON(http.StatusBadRequest, ChatbotResponse{
				Error: "Message cannot be empty",
			})
			return
		}

		webSearch := true
		if req.WebSearch != nil {
			webSearch = *req.WebSearch
		}

		response, err := services.GetAgentResponse(req.Message, webSearch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ChatbotResponse{
				Error: "Failed to generate response",
			})
			return
		}

		c.JSON(http.StatusOK, ChatbotResponse{
			Reply:            response.Reply,
			Sources:          response.Sources,
			SourceCount:      response.SourceCount,
			Model:            response.Model,
			WebSearchEnabled: response.WebSearchEnabled,
		})
	}
}
