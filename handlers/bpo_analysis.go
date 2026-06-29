package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solusphere_backend/models"
	"solusphere_backend/services"
)

type BPOAnalysisHandler struct {
	db            *sql.DB
	pdfProcessor  *services.PDFProcessor
	uploadService *services.UploadService
}

func NewBPOAnalysisHandler(db *sql.DB, pdfProcessor *services.PDFProcessor, uploadService *services.UploadService) *BPOAnalysisHandler {
	return &BPOAnalysisHandler{
		db:            db,
		pdfProcessor:  pdfProcessor,
		uploadService: uploadService,
	}
}

// UploadAndAnalyzePDF handles PDF upload and analysis
func (h *BPOAnalysisHandler) UploadAndAnalyzePDF(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireDocumentProcessingConsent(c, userID) {
		return
	}

	file, err := c.FormFile("document")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No document file provided",
		})
		return
	}

	// Validate file type
	if filepath.Ext(file.Filename) != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Only PDF files are supported",
		})
		return
	}

	// Validate file size (max 20MB)
	if file.Size > 20*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File size too large. Maximum size is 20MB",
		})
		return
	}

	// Upload file
	filePath, err := h.uploadService.SaveUploadedFile(file)
	if err != nil {
		log.Printf("Failed to save uploaded file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save uploaded file: " + err.Error(),
		})
		return
	}

	// Create analysis record
	analysis := &models.BPOAnalysis{
		ID:        uuid.New().String(),
		Filename:  file.Filename,
		FilePath:  filePath,
		FileSize:  file.Size,
		MimeType:  file.Header.Get("Content-Type"),
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := models.CreateBPOAnalysis(h.db, analysis); err != nil {
		log.Printf("Failed to create analysis record: %v", err)
		// Clean up uploaded file if database operation fails
		os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create analysis record: " + err.Error(),
		})
		return
	}

	// Process analysis asynchronously
	go h.processAnalysis(analysis.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "PDF uploaded and analysis started",
		"analysis_id": analysis.ID,
		"status":      analysis.Status,
		"filename":    analysis.Filename,
		"uploaded_at": time.Now().Format(time.RFC3339),
	})
}

// GetAnalysisResult retrieves analysis results
func (h *BPOAnalysisHandler) GetAnalysisResult(c *gin.Context) {
	analysisID := c.Param("id")

	if analysisID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Analysis ID is required",
		})
		return
	}

	analysis, err := models.GetBPOAnalysisByID(h.db, analysisID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve analysis: " + err.Error(),
		})
		return
	}

	if analysis == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Analysis not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analysis": analysis,
	})
}

// ListAnalyses returns paginated list of analyses
func (h *BPOAnalysisHandler) ListAnalyses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")
	analysisType := c.Query("type")

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	analyses, total, err := models.ListBPOAnalyses(h.db, page, limit, status, analysisType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve analyses: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analyses": analyses,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

// DeleteAnalysis deletes an analysis record
func (h *BPOAnalysisHandler) DeleteAnalysis(c *gin.Context) {
	analysisID := c.Param("id")

	if analysisID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Analysis ID is required",
		})
		return
	}

	// Get analysis first to get file path
	analysis, err := models.GetBPOAnalysisByID(h.db, analysisID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to find analysis: " + err.Error(),
		})
		return
	}

	if analysis == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Analysis not found",
		})
		return
	}

	// Delete the file from filesystem
	if err := os.Remove(analysis.FilePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Failed to delete file %s: %v", analysis.FilePath, err)
	}

	// Delete from database
	if err := models.DeleteBPOAnalysis(h.db, analysisID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete analysis: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Analysis deleted successfully",
	})
}

// processAnalysis handles the actual PDF processing and analysis
func (h *BPOAnalysisHandler) processAnalysis(analysisID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Update status to processing
	analysis, err := models.GetBPOAnalysisByID(h.db, analysisID)
	if err != nil {
		log.Printf("Error finding analysis record: %v", err)
		return
	}

	if analysis == nil {
		log.Printf("Analysis record not found: %s", analysisID)
		return
	}

	analysis.Status = models.StatusProcessing
	analysis.UpdatedAt = time.Now()
	if err := models.UpdateBPOAnalysis(h.db, analysis); err != nil {
		log.Printf("Error updating analysis status to processing: %v", err)
		return
	}

	// Extract text from PDF
	extractedText, pageCount, err := h.pdfProcessor.ExtractTextFromPDF(analysis.FilePath)
	if err != nil {
		log.Printf("Error extracting text from PDF: %v", err)
		h.updateAnalysisStatus(analysisID, models.StatusFailed, err.Error())
		return
	}

	// Detect document type
	docType := h.pdfProcessor.DetectDocumentType(extractedText)

	// Analyze content with OpenAI
	analysisResult, err := h.pdfProcessor.AnalyzeBPOContent(ctx, extractedText, docType)
	if err != nil {
		log.Printf("Error analyzing content: %v", err)
		h.updateAnalysisStatus(analysisID, models.StatusFailed, err.Error())
		return
	}

	// Convert analysis result to JSON string
	analysisResultJSON, err := models.AnalysisResultToJSON(analysisResult)
	if err != nil {
		log.Printf("Error converting analysis result to JSON: %v", err)
		h.updateAnalysisStatus(analysisID, models.StatusFailed, err.Error())
		return
	}

	// Update analysis record with results
	analysis.Status = models.StatusCompleted
	analysis.ExtractedText = extractedText
	analysis.AnalysisResult = analysisResultJSON
	analysis.PageCount = pageCount
	analysis.AnalysisType = docType
	analysis.ConfidenceScore = getConfidenceScore(analysisResult)
	analysis.UpdatedAt = time.Now()

	if err := models.UpdateBPOAnalysis(h.db, analysis); err != nil {
		log.Printf("Error updating analysis record: %v", err)
	}

	log.Printf("Analysis completed for document: %s (ID: %s)", analysis.Filename, analysis.ID)
}

// helper function to update analysis status
func (h *BPOAnalysisHandler) updateAnalysisStatus(analysisID string, status string, errorMsg ...string) {
	analysis, err := models.GetBPOAnalysisByID(h.db, analysisID)
	if err != nil {
		log.Printf("Error finding analysis record for status update: %v", err)
		return
	}

	if analysis == nil {
		log.Printf("Analysis record not found for status update: %s", analysisID)
		return
	}

	analysis.Status = status
	analysis.UpdatedAt = time.Now()

	if len(errorMsg) > 0 && status == models.StatusFailed {
		errorResult := map[string]interface{}{"error": errorMsg[0]}
		errorJSON, err := models.AnalysisResultToJSON(errorResult)
		if err != nil {
			log.Printf("Error creating error result JSON: %v", err)
		} else {
			analysis.AnalysisResult = errorJSON
		}
	}

	if err := models.UpdateBPOAnalysis(h.db, analysis); err != nil {
		log.Printf("Error updating analysis status: %v", err)
	}
}

// helper function to extract confidence score from analysis result
func getConfidenceScore(analysisResult map[string]interface{}) float64 {
	if extractedData, ok := analysisResult["extracted_data"].(map[string]interface{}); ok {
		if confidence, ok := extractedData["confidence_score"].(float64); ok && confidence > 0 {
			return confidence
		}
		if confidence, ok := extractedData["confidence"].(float64); ok && confidence > 0 {
			return confidence
		}
	}
	return 0.75
}
