package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

// SetTyping POST /api/chats/:user_id/typing — call every few seconds while composing.
// Body {"typing": false} clears it (for example when the box is emptied).
func SetTyping(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	peerID, ok := parseChatUserID(c)
	if !ok || peerID == userID {
		if ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		}
		return
	}
	var body struct {
		Typing *bool `json:"typing"`
	}
	_ = c.ShouldBindJSON(&body)

	var err error
	if body.Typing != nil && !*body.Typing {
		err = models.ClearTyping(database.DB, userID, peerID)
	} else {
		err = models.TouchTyping(database.DB, userID, peerID)
	}
	if err != nil {
		if database.IsMissingTableError(err) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update typing status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetTyping GET /api/chats/:user_id/typing — whether that user is typing to me right now.
func GetTyping(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	peerID, ok := parseChatUserID(c)
	if !ok {
		return
	}
	setNoCacheHeaders(c)
	typing, err := models.IsTyping(database.DB, peerID, userID)
	if err != nil && !database.IsMissingTableError(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read typing status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"typing": typing})
}
