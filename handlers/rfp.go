package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solusphere_backend/database"
	"solusphere_backend/models"
	"solusphere_backend/services"
)

const (
	rfpMaxFiles    = 12
	rfpMaxFileSize = 15 << 20
)

type rfpHTTPError struct {
	status int
	msg    string
}

func (e *rfpHTTPError) Error() string { return e.msg }

func rfpBadRequest(msg string) error { return &rfpHTTPError{status: http.StatusBadRequest, msg: msg} }

func rfpRespondError(c *gin.Context, err error) {
	if he, ok := err.(*rfpHTTPError); ok {
		c.JSON(he.status, gin.H{"error": he.msg})
		return
	}
	log.Printf("RFP: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong."})
}

// rfpSourcesFromRequest reads the multipart "files" field (also accepts "document").
func rfpSourcesFromRequest(c *gin.Context) ([]models.RFPSource, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, rfpBadRequest("Choose at least one document.")
	}
	files := append(form.File["files"], form.File["document"]...)
	if len(files) == 0 {
		return nil, rfpBadRequest("Choose at least one document.")
	}
	if len(files) > rfpMaxFiles {
		return nil, rfpBadRequest("You can add up to 12 documents.")
	}

	sources := []models.RFPSource{}
	for _, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext != ".pdf" && ext != ".docx" && ext != ".txt" && ext != ".md" {
			return nil, rfpBadRequest("Upload a PDF, Word (.docx), or text file.")
		}
		if fh.Size > rfpMaxFileSize {
			return nil, rfpBadRequest("Each file needs to be 15 MB or smaller.")
		}
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		text, err := services.ExtractRFPSourceText(fh.Filename, data)
		if err != nil {
			log.Printf("RFP: could not read %s: %v", fh.Filename, err)
			continue
		}
		if len([]rune(text)) < 20 {
			continue
		}
		sources = append(sources, models.RFPSource{Name: filepath.Base(fh.Filename), Text: text})
	}
	if len(sources) == 0 {
		return nil, rfpBadRequest("Those files do not contain enough text. If a PDF is a scan, export a text-based copy or upload a Word file.")
	}
	return sources, nil
}

func combineRFPSources(sources []models.RFPSource) (string, bool) {
	var body string
	if len(sources) == 1 {
		body = sources[0].Text
	} else {
		parts := make([]string, 0, len(sources))
		for i, s := range sources {
			parts = append(parts, fmt.Sprintf("Document %d: %s\n%s", i+1, s.Name, s.Text))
		}
		body = strings.Join(parts, "\n\n")
	}
	runes := []rune(body)
	if len(runes) > services.RFPMaxCombinedRunes {
		return string(runes[:services.RFPMaxCombinedRunes]), true
	}
	return body, false
}

func rfpSourceLabel(sources []models.RFPSource) string {
	names := make([]string, 0, len(sources))
	for _, s := range sources {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

// queueRFPAnalysis marks the document as processing, saves it, and analyses it in the background.
// Analysis can take minutes with long tenders, longer than the CloudFront/ALB request timeout,
// so the client polls GET /api/rfp/documents/:id until status leaves "processing".
func queueRFPAnalysis(doc *models.RFPDocument) error {
	if len([]rune(doc.ExtractedText)) < 40 {
		return rfpBadRequest("Those files do not contain enough text to build a template.")
	}
	doc.Status = models.RFPStatusProcessing
	doc.Warning = ""
	if err := models.SaveRFPDocument(database.DB, doc); err != nil {
		return err
	}

	snapshot := *doc
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("RFP analysis panic for %s: %v", snapshot.ID, r)
				snapshot.Status = models.RFPStatusFailed
				snapshot.Warning = "The template could not be built. Try again."
				_ = models.SaveRFPDocument(database.DB, &snapshot)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		result := services.AnalyzeRFPText(ctx, snapshot.ExtractedText, snapshot.OriginalFilename)
		warnings := []string{}
		if result.Warning != "" {
			warnings = append(warnings, result.Warning)
		}
		if snapshot.Truncated {
			warnings = append(warnings, "The combined documents were very long, so the last part was not included.")
		}
		snapshot.Template = models.NormalizeRFPTemplate(result.Template)
		snapshot.Source = result.Source
		snapshot.Model = result.Model
		snapshot.Warning = strings.Join(warnings, " ")
		snapshot.Status = models.RFPDocumentStatus(snapshot.Template)
		if err := models.SaveRFPDocument(database.DB, &snapshot); err != nil {
			log.Printf("RFP: save analysis for %s failed: %v", snapshot.ID, err)
		}
	}()
	return nil
}

func loadOwnedRFP(c *gin.Context) (*models.RFPDocument, bool) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, false
	}
	doc, err := models.GetRFPDocument(database.DB, c.Param("id"), userID)
	if err != nil {
		rfpRespondError(c, err)
		return nil, false
	}
	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found."})
		return nil, false
	}
	return doc, true
}

// ListRFPDocuments GET /api/rfp/documents
func ListRFPDocuments(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	docs, err := models.ListRFPDocuments(database.DB, userID)
	if err != nil {
		rfpRespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": docs, "aiConfigured": services.IsOpenAIInitialized()})
}

// GetRFPDocument GET /api/rfp/documents/:id
func GetRFPDocument(c *gin.Context) {
	doc, ok := loadOwnedRFP(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, doc.Present())
}

// CreateRFPDocument POST /api/rfp/documents (multipart "files")
func CreateRFPDocument(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireDocumentProcessingConsent(c, userID) {
		return
	}
	sources, err := rfpSourcesFromRequest(c)
	if err != nil {
		rfpRespondError(c, err)
		return
	}
	text, truncated := combineRFPSources(sources)
	doc := &models.RFPDocument{
		ID:               uuid.NewString(),
		UserID:           userID,
		OriginalFilename: rfpSourceLabel(sources),
		Sources:          sources,
		ExtractedText:    text,
		Source:           "builtin",
		Truncated:        truncated,
	}
	if err := queueRFPAnalysis(doc); err != nil {
		rfpRespondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, doc.Present())
}

// AddRFPSources POST /api/rfp/documents/:id/sources (multipart "files")
func AddRFPSources(c *gin.Context) {
	doc, ok := loadOwnedRFP(c)
	if !ok {
		return
	}
	if !requireDocumentProcessingConsent(c, doc.UserID) {
		return
	}
	if doc.Status == models.RFPStatusProcessing {
		c.JSON(http.StatusConflict, gin.H{"error": "This template is still being built. Try again when it is ready."})
		return
	}
	added, err := rfpSourcesFromRequest(c)
	if err != nil {
		rfpRespondError(c, err)
		return
	}
	current := doc.Sources
	if len(current) == 0 {
		current = []models.RFPSource{{Name: doc.OriginalFilename, Text: doc.ExtractedText}}
	}
	sources := append(current, added...)
	if len(sources) > rfpMaxFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A template can include up to 12 documents."})
		return
	}
	doc.Sources = sources
	doc.OriginalFilename = rfpSourceLabel(sources)
	doc.ExtractedText, doc.Truncated = combineRFPSources(sources)
	if err := queueRFPAnalysis(doc); err != nil {
		rfpRespondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, doc.Present())
}

// ReanalyzeRFPDocument POST /api/rfp/documents/:id/reanalyze
func ReanalyzeRFPDocument(c *gin.Context) {
	doc, ok := loadOwnedRFP(c)
	if !ok {
		return
	}
	if doc.Status == models.RFPStatusProcessing {
		c.JSON(http.StatusConflict, gin.H{"error": "This template is already being built."})
		return
	}
	if err := queueRFPAnalysis(doc); err != nil {
		rfpRespondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, doc.Present())
}

// UpdateRFPDocument PUT /api/rfp/documents/:id  body: {"template": {...}}
func UpdateRFPDocument(c *gin.Context) {
	doc, ok := loadOwnedRFP(c)
	if !ok {
		return
	}
	if doc.Status == models.RFPStatusProcessing {
		c.JSON(http.StatusConflict, gin.H{"error": "This template is still being built."})
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template."})
		return
	}
	raw, ok := body["template"].(map[string]interface{})
	if !ok {
		raw = body
	}
	doc.Template = models.NormalizeRFPTemplateMap(raw)
	doc.Status = models.RFPDocumentStatus(doc.Template)
	if err := models.SaveRFPDocument(database.DB, doc); err != nil {
		rfpRespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc.Present())
}

// DeleteRFPDocument DELETE /api/rfp/documents/:id
func DeleteRFPDocument(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	removed, err := models.DeleteRFPDocument(database.DB, c.Param("id"), userID)
	if err != nil {
		rfpRespondError(c, err)
		return
	}
	if !removed {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DownloadRFPDocument GET /api/rfp/documents/:id/download
func DownloadRFPDocument(c *gin.Context) {
	doc, ok := loadOwnedRFP(c)
	if !ok {
		return
	}
	data, err := services.RenderRFPDocx(doc)
	if err != nil {
		rfpRespondError(c, err)
		return
	}
	filename := services.RFPSlug(doc.Template.Title) + "-rfp-template.docx"
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
}
