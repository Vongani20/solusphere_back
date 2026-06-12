package middleware

import (
	"net/http"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

const faceRegistrationPath = "/api/face/register"

var faceRegistrationExemptPaths = map[string]struct{}{
	"/api/profile":       {},
	faceRegistrationPath: {},
	"/api/face/update":   {},
	"/api/face/delete":   {},
}

// RequireCompletedFaceRegistration blocks protected app endpoints until a user
// has completed face registration after first password login.
func RequireCompletedFaceRegistration() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exempt := faceRegistrationExemptPaths[c.FullPath()]; exempt {
			c.Next()
			return
		}

		userID := c.GetInt("userID")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		registered, _, err := models.GetUserFaceRegistrationStatus(database.DB, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to check face registration status",
			})
			c.Abort()
			return
		}

		if !registered {
			c.JSON(http.StatusPreconditionRequired, FaceRegistrationRequiredBody())
			c.Abort()
			return
		}

		c.Set("face_registered", true)
		c.Next()
	}
}

func FaceRegistrationRequiredBody() gin.H {
	return gin.H{
		"error":         "Face registration required",
		"code":          "FACE_REGISTRATION_REQUIRED",
		"message":       "Register your face before using this endpoint.",
		"face_required": true,
		"face_status":   false,
		"next_step":     "POST " + faceRegistrationPath,
	}
}
