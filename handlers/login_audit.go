package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

func recordLoginAudit(c *gin.Context, entry models.LoginAuditLog) {
	entry.IPAddress = c.ClientIP()
	entry.UserAgent = c.GetHeader("User-Agent")
	if err := models.CreateLoginAuditLog(database.DB, &entry); err != nil {
		log.Printf("Failed to record login audit: %v", err)
	}
}

func recordPasswordLoginFailure(c *gin.Context, email string, user *models.User, reason string) {
	entry := models.LoginAuditLog{
		Email:         strings.TrimSpace(strings.ToLower(email)),
		LoginMethod:   models.LoginMethodPassword,
		Status:        models.LoginStatusFailed,
		FailureReason: reason,
	}
	if user != nil {
		entry.UserID = &user.ID
		entry.Username = user.Username
		if entry.Email == "" {
			entry.Email = user.Email
		}
	}
	recordLoginAudit(c, entry)
}

func recordPasswordLoginSuccess(c *gin.Context, user *models.User) {
	if user == nil {
		return
	}
	userID := user.ID
	recordLoginAudit(c, models.LoginAuditLog{
		UserID:      &userID,
		Email:       user.Email,
		Username:    user.Username,
		LoginMethod: models.LoginMethodPassword,
		Status:      models.LoginStatusSuccess,
	})
}

func recordFaceLoginFailure(c *gin.Context, reason string) {
	recordLoginAudit(c, models.LoginAuditLog{
		LoginMethod:   models.LoginMethodFace,
		Status:        models.LoginStatusFailed,
		FailureReason: reason,
	})
}

func recordFaceLoginSuccess(c *gin.Context, user models.UserMinimal) {
	userID := user.ID
	recordLoginAudit(c, models.LoginAuditLog{
		UserID:      &userID,
		Email:       user.Email,
		Username:    user.Username,
		LoginMethod: models.LoginMethodFace,
		Status:      models.LoginStatusSuccess,
	})
}

// ListLoginAuditLogsByAdmin returns paginated login audit entries.
func ListLoginAuditLogsByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	filterUserID, _ := strconv.Atoi(c.Query("user_id"))

	logs, total, err := models.ListLoginAuditLogs(database.DB, models.LoginAuditFilter{
		Email:  c.Query("email"),
		Status: c.Query("status"),
		Method: c.Query("method"),
		UserID: filterUserID,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load login audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs": logs,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}
