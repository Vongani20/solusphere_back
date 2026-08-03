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

type cvBuilderLogRequest struct {
	Action    string `json:"action" binding:"required"`
	FromStep  *int   `json:"from_step"`
	ToStep    *int   `json:"to_step"`
	FromLabel string `json:"from_label"`
	ToLabel   string `json:"to_label"`
	Format    string `json:"format"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

func recordCVBuilderLog(c *gin.Context, entry models.CVBuilderLog) {
	entry.IPAddress = c.ClientIP()
	entry.UserAgent = c.GetHeader("User-Agent")
	if err := models.CreateCVBuilderLog(database.DB, &entry); err != nil {
		log.Printf("Failed to record CV builder log: %v", err)
	}
}

func enrichCVBuilderLogUser(entry *models.CVBuilderLog) {
	if entry == nil || entry.UserID <= 0 {
		return
	}
	if entry.Email != "" && entry.Username != "" {
		return
	}
	user, err := models.GetUserByID(database.DB, entry.UserID)
	if err != nil || user == nil {
		return
	}
	if entry.Email == "" {
		entry.Email = user.Email
	}
	if entry.Username == "" {
		entry.Username = user.Username
	}
}

// CreateCVBuilderLog records a CV Builder UI click (next step / download).
func CreateCVBuilderLog(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req cvBuilderLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	action := strings.TrimSpace(strings.ToLower(req.Action))
	switch action {
	case models.CVBuilderActionNextStep, models.CVBuilderActionDownloadPDF, models.CVBuilderActionDownloadWord:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be next_step, download_pdf, or download_word"})
		return
	}

	status := strings.TrimSpace(strings.ToLower(req.Status))
	if status == "" {
		status = models.CVBuilderStatusClicked
	}

	format := strings.TrimSpace(strings.ToLower(req.Format))
	if action == models.CVBuilderActionDownloadPDF && format == "" {
		format = "pdf"
	}
	if action == models.CVBuilderActionDownloadWord && format == "" {
		format = "word"
	}

	entry := models.CVBuilderLog{
		UserID:    userID,
		Action:    action,
		FromStep:  req.FromStep,
		ToStep:    req.ToStep,
		FromLabel: strings.TrimSpace(req.FromLabel),
		ToLabel:   strings.TrimSpace(req.ToLabel),
		Format:    format,
		Status:    status,
		Detail:    strings.TrimSpace(req.Detail),
	}
	enrichCVBuilderLogUser(&entry)
	recordCVBuilderLog(c, entry)

	c.JSON(http.StatusCreated, gin.H{"message": "CV builder event logged"})
}

// ListCVBuilderLogsByAdmin returns paginated CV Builder activity logs.
func ListCVBuilderLogsByAdmin(c *gin.Context) {
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

	logs, total, err := models.ListCVBuilderLogs(database.DB, models.CVBuilderLogFilter{
		Email:  c.Query("email"),
		Action: c.Query("action"),
		UserID: filterUserID,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CV builder logs"})
		return
	}

	totalPages := 1
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}
	if page < 1 {
		page = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"logs": logs,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}
