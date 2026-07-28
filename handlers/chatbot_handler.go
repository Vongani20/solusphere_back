package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"solusphere_backend/internal/ai"
	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

type ChatbotImageInput struct {
	DataURL string `json:"data_url"`
	Name    string `json:"name,omitempty"`
}

type ChatbotRequest struct {
	Message   string              `json:"message" binding:"required"`
	WebSearch *bool               `json:"web_search,omitempty"`
	Images    []ChatbotImageInput `json:"images,omitempty"`
}

type ChatbotResponse struct {
	Reply            string        `json:"reply"`
	Sources          []ai.Citation `json:"sources,omitempty"`
	SourceCount      int           `json:"source_count"`
	Model            string        `json:"model,omitempty"`
	WebSearchEnabled bool          `json:"web_search_enabled"`
	Error            string        `json:"error,omitempty"`
}

const (
	maxChatbotImages     = 6
	maxChatbotImageBytes = 4 << 20 // data URL string budget (~3MB raw image)
)

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

		images, err := normalizeChatbotImages(req.Images)
		if err != nil {
			c.JSON(http.StatusBadRequest, ChatbotResponse{Error: err.Error()})
			return
		}

		response, err := services.GetAgentResponse(req.Message, webSearch, images)
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

func normalizeChatbotImages(raw []ChatbotImageInput) ([]ai.ImageInput, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > maxChatbotImages {
		return nil, fmt.Errorf("you can attach at most 6 images per message")
	}

	out := make([]ai.ImageInput, 0, len(raw))
	for _, item := range raw {
		dataURL := strings.TrimSpace(item.DataURL)
		if dataURL == "" {
			continue
		}
		if !strings.HasPrefix(dataURL, "data:image/") || !strings.Contains(dataURL, ";base64,") {
			return nil, fmt.Errorf("each image must be a data:image/...;base64,... URL")
		}
		if len(dataURL) > maxChatbotImageBytes {
			return nil, fmt.Errorf("one or more images are too large (max ~3 MB each)")
		}
		out = append(out, ai.ImageInput{
			ImageURL: dataURL,
			Detail:   "auto",
		})
	}
	return out, nil
}
