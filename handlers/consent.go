package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

type signConsentRequest struct {
	ConsentType string `json:"consent_type" binding:"required"`
	SignedName  string `json:"signed_name" binding:"required"`
	Accept      bool   `json:"accept"`
}

// GetUserConsents returns consent status for the authenticated user.
func GetUserConsents(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	consents, err := models.ListUserConsentStatus(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load consent status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"consents": consents})
}

// SignUserConsent records the user's signed consent.
func SignUserConsent(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req signConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if !req.Accept {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You must accept the consent terms to continue"})
		return
	}

	consentType := strings.TrimSpace(req.ConsentType)
	signedName := strings.TrimSpace(req.SignedName)
	if signedName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Signed name is required"})
		return
	}

	version, ok := consentVersionForType(consentType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown consent type"})
		return
	}

	consent := &models.UserConsent{
		UserID:         userID,
		ConsentType:    consentType,
		ConsentVersion: version,
		SignedName:     signedName,
		IPAddress:      c.ClientIP(),
		UserAgent:      c.GetHeader("User-Agent"),
	}
	if err := models.RecordUserConsent(database.DB, consent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record consent"})
		return
	}

	consents, err := models.ListUserConsentStatus(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Consent recorded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Consent recorded",
		"consents": consents,
	})
}

func consentVersionForType(consentType string) (string, bool) {
	switch consentType {
	case models.ConsentTypeAIDocumentProcessing:
		return models.ConsentVersionAIDocument, true
	default:
		return "", false
	}
}

func requireDocumentProcessingConsent(c *gin.Context, userID int) bool {
	has, err := models.HasUserConsent(
		database.DB,
		userID,
		models.ConsentTypeAIDocumentProcessing,
		models.ConsentVersionAIDocument,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify consent"})
		return false
	}
	if has {
		return true
	}

	c.JSON(http.StatusForbidden, gin.H{
		"error":        "Document processing consent is required before uploading documents",
		"code":         "consent_required",
		"consent_type": models.ConsentTypeAIDocumentProcessing,
	})
	return false
}
