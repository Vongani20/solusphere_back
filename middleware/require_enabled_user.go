package middleware

import (
	"net/http"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

// RequireEnabledUser rejects requests from accounts an admin has disabled.
func RequireEnabledUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("userID")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}

		disabled, err := models.IsUserDisabled(database.DB, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify account status"})
			c.Abort()
			return
		}
		if disabled {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "This account has been disabled. Contact an administrator.",
				"code":  "ACCOUNT_DISABLED",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
