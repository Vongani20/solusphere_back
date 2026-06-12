package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type directMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

func ListChatUsers(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	users, err := models.ListChatUsers(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

func ListDirectConversations(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	conversations, err := models.ListDirectConversations(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load conversations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func ListDirectMessages(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	otherUserID, ok := parseChatUserID(c)
	if !ok {
		return
	}
	if otherUserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot open a direct chat with yourself"})
		return
	}
	if !ensureUserExists(c, otherUserID) {
		return
	}

	limit := 50
	if rawLimit := c.Query("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsedLimit
		}
	}

	messages, err := models.ListDirectMessages(database.DB, userID, otherUserID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func SendDirectMessage(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	receiverID, ok := parseChatUserID(c)
	if !ok {
		return
	}
	if receiverID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send a direct message to yourself"})
		return
	}
	if !ensureUserExists(c, receiverID) {
		return
	}

	var req directMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	body := strings.TrimSpace(req.Message)
	if body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message is required"})
		return
	}
	if len(body) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message must be 4000 characters or fewer"})
		return
	}

	message, err := models.CreateDirectMessage(database.DB, userID, receiverID, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": message})
}

func ensureUserExists(c *gin.Context, userID int) bool {
	user, err := models.GetUserByID(database.DB, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user"})
		return false
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return false
	}
	return true
}

func parseChatUserID(c *gin.Context) (int, bool) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return 0, false
	}
	return userID, true
}
