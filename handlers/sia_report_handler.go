package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

type SIAReportRequest struct {
	Prompt    string `json:"prompt" binding:"required"`
	Format    string `json:"format" binding:"required"`
	WebSearch *bool  `json:"web_search,omitempty"`
}

// SIAReportHandler generates a Word, Excel, or PowerPoint research report from a user prompt.
func SIAReportHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !services.IsOpenAIInitialized() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "OpenAI is not configured. Set OPENAI_API_KEY to enable SIA report generation.",
			})
			return
		}

		var req SIAReportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		req.Prompt = strings.TrimSpace(req.Prompt)
		if req.Prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt cannot be empty"})
			return
		}

		format, err := services.ParseReportFormat(req.Format)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		webSearch := true
		if req.WebSearch != nil {
			webSearch = *req.WebSearch
		}

		report, err := services.GenerateSIAReport(req.Prompt, format, webSearch)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate report", "details": err.Error()})
			return
		}

		c.Header("Content-Type", report.ContentType)
		c.Header("Content-Disposition", `attachment; filename="`+report.Filename+`"`)
		c.Header("X-SIA-Model", report.Model)
		c.Header("X-SIA-Source-Count", strconv.Itoa(report.SourceCount))
		c.Data(http.StatusOK, report.ContentType, report.Data)
	}
}
