package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

type reactionRequest struct {
	Emoji string `json:"emoji"`
}

// ReactToDirectMessage POST /api/chats/messages/:message_id/reactions {"emoji":"👍"}
// Sending the same emoji again removes the reaction.
func ReactToDirectMessage(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	messageID, err := strconv.Atoi(c.Param("message_id"))
	if err != nil || messageID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}
	var req reactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	reactions, err := models.ToggleMessageReaction(database.DB, messageID, userID, req.Emoji)
	switch {
	case errors.Is(err, models.ErrInvalidReaction):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, sql.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reaction"})
	default:
		c.JSON(http.StatusOK, gin.H{"message_id": messageID, "reactions": reactions})
	}
}
