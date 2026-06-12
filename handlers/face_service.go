package handlers

import (
	"net/http"

	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

func ensureRekognitionConfigured(c *gin.Context, rekogSvc *services.RekognitionService) bool {
	if rekogSvc != nil && rekogSvc.Client != nil {
		return true
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "Face recognition is not configured",
		"message": "Set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION, and AWS_BUCKET_NAME to enable face recognition",
	})
	return false
}
