package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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

func parseCVBuilderLogFilter(c *gin.Context, page, limit int, export bool) models.CVBuilderLogFilter {
	filterUserID, _ := strconv.Atoi(c.Query("user_id"))
	return models.CVBuilderLogFilter{
		Email:  c.Query("email"),
		Action: c.Query("action"),
		UserID: filterUserID,
		Page:   page,
		Limit:  limit,
		Export: export,
	}
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

	logs, total, err := models.ListCVBuilderLogs(database.DB, parseCVBuilderLogFilter(c, page, limit, false))
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

// DownloadCVBuilderLogsTXT exports matching CV Builder logs as a plain-text file.
func DownloadCVBuilderLogsTXT(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	logs, total, err := models.ListCVBuilderLogs(database.DB, parseCVBuilderLogFilter(c, 1, 10000, true))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export CV builder logs"})
		return
	}

	var b strings.Builder
	b.WriteString("Solusphere CV Builder Logs\n")
	b.WriteString(fmt.Sprintf("Exported at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Total records: %d (export limit 10000)\n", total))
	if email := strings.TrimSpace(c.Query("email")); email != "" {
		b.WriteString(fmt.Sprintf("Filter email: %s\n", email))
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		b.WriteString(fmt.Sprintf("Filter action: %s\n", action))
	}
	b.WriteString(strings.Repeat("-", 100) + "\n")
	b.WriteString(fmt.Sprintf("%-20s  %-8s  %-28s  %-14s  %-10s  %-28s  %-16s  %s\n",
		"WHEN_UTC", "USER_ID", "EMAIL", "ACTION", "STATUS", "STEP", "IP", "DETAIL"))
	b.WriteString(strings.Repeat("-", 100) + "\n")

	for _, entry := range logs {
		step := "—"
		if entry.Action == models.CVBuilderActionNextStep {
			from := entry.FromLabel
			to := entry.ToLabel
			if from == "" && entry.FromStep != nil {
				from = fmt.Sprintf("step %d", *entry.FromStep)
			}
			if to == "" && entry.ToStep != nil {
				to = fmt.Sprintf("step %d", *entry.ToStep)
			}
			if from == "" {
				from = "—"
			}
			if to == "" {
				to = "—"
			}
			step = from + " -> " + to
		} else if entry.FromLabel != "" {
			step = entry.FromLabel
		} else if entry.Format != "" {
			step = entry.Format
		}

		email := entry.Email
		if email == "" {
			email = "—"
		}
		ip := entry.IPAddress
		if ip == "" {
			ip = "—"
		}
		detail := entry.Detail
		if detail == "" {
			detail = "—"
		}
		detail = strings.ReplaceAll(detail, "\n", " ")
		detail = strings.ReplaceAll(detail, "\r", " ")

		b.WriteString(fmt.Sprintf("%-20s  %-8d  %-28s  %-14s  %-10s  %-28s  %-16s  %s\n",
			entry.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
			entry.UserID,
			truncateTXT(email, 28),
			truncateTXT(entry.Action, 14),
			truncateTXT(entry.Status, 10),
			truncateTXT(step, 28),
			truncateTXT(ip, 16),
			detail,
		))
	}

	filename := fmt.Sprintf("cv_builder_logs_%s.txt", time.Now().UTC().Format("20060102_150405"))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.String(http.StatusOK, b.String())
}

func truncateTXT(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
