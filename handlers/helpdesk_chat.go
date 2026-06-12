package handlers

import (
	"net/http"
	"solusphere_backend/models"
	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

func HelpdeskChatHandler(c *gin.Context) {
	var req models.ChatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := services.GetBPOResponse(req.UserMessage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ChatResponse{
		AgentMessage: reply,
	})
}
