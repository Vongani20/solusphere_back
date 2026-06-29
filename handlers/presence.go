package handlers

import (
	"net/http"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

func TouchPresence(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := models.TouchUserPresence(database.DB, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update presence"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ListUserPresence(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	setNoCacheHeaders(c)

	presence, err := models.ListUserPresence(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load presence"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"presence": presence})
}
