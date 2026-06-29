package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/models"
	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

var cvPDFService = services.NewCVPDFService()

// GetCV returns the authenticated user's CV data.
func GetCV(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	profile, err := models.GetCVProfileByUserID(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CV"})
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CV not found"})
		return
	}
	enrichCVPhotoURL(profile)
	c.JSON(http.StatusOK, gin.H{"cv": profile})
}

// CreateOrUpdateCV upserts the authenticated user's CV data.
func CreateOrUpdateCV(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireDocumentProcessingConsent(c, userID) {
		return
	}

	var profile models.CVProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	profile.UserID = userID
	models.SanitizeCVProfile(&profile)

	if errs := validateCVProfile(&profile); len(errs) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "fields": errs})
		return
	}

	// Preserve existing photo URL — only changed via /cv/photo
	existing, _ := models.GetCVProfileByUserID(database.DB, userID)
	if existing != nil {
		profile.ProfilePhotoURL = existing.ProfilePhotoURL
	}

	if err := models.UpsertCVProfile(database.DB, &profile); err != nil {
		log.Printf("CV save failed for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save CV", "details": err.Error()})
		return
	}

	saved, err := models.GetCVProfileByUserID(database.DB, userID)
	if err != nil || saved == nil {
		c.JSON(http.StatusOK, gin.H{"message": "CV saved"})
		return
	}
	enrichCVPhotoURL(saved)
	c.JSON(http.StatusOK, gin.H{"cv": saved})
}

// DeleteCV removes the authenticated user's CV and any stored profile photo.
func DeleteCV(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	existing, err := models.GetCVProfileByUserID(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CV"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CV not found"})
		return
	}

	if existing.ProfilePhotoURL != "" {
		if key, ok := models.S3KeyFromObjectURL(existing.ProfilePhotoURL); ok {
			_ = models.DeleteFromS3(key)
		}
	}

	deleted, err := models.DeleteCVProfileByUserID(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete CV"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "CV not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "CV deleted"})
}

// validateCVProfile checks the personal fields required by the CV builder wizard.
// Skills, qualifications, languages, and experience may be saved incrementally while
// the user moves through the multi-step form.
func validateCVProfile(p *models.CVProfile) map[string]string {
	errs := map[string]string{}

	if p.FirstName == "" {
		errs["first_name"] = "First name is required"
	}
	if p.LastName == "" {
		errs["last_name"] = "Last name is required"
	}
	if p.Gender == "" {
		errs["gender"] = "Gender is required"
	}
	if p.Nationality == "" {
		errs["nationality"] = "Nationality is required"
	}
	if p.DateOfBirth == "" {
		errs["date_of_birth"] = "Date of birth is required"
	}
	if p.ProfileText == "" {
		errs["profile_text"] = "Profile is required"
	}
	if p.ValueProposition == "" {
		errs["value_proposition"] = "Value proposition is required"
	}

	return errs
}

// ImportCVFromDocument parses an uploaded CV PDF and returns structured fields for the wizard.
func ImportCVFromDocument(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "CV document is required"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only PDF files are supported"})
		return
	}

	const maxSize = 10 << 20 // 10 MB
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be smaller than 10 MB"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read uploaded file"})
		return
	}

	text, _, err := services.ExtractTextFromPDFBytes(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not read text from PDF. Try a text-based PDF export."})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	profile, warnings, err := services.ParseCVFromDocumentText(ctx, text)
	if err != nil {
		log.Printf("CV import failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse CV document", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cv":       profile,
		"warnings": warnings,
		"message":  "CV imported. Review the form and save when ready.",
	})
}

// UploadCVPhoto handles profile photo upload, stores in S3, and updates the CV record.
func UploadCVPhoto(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireDocumentProcessingConsent(c, userID) {
		return
	}

	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Photo file is required"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only JPG and PNG photos are accepted"})
		return
	}

	const maxSize = 5 << 20 // 5 MB
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Photo must be smaller than 5 MB"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read photo"})
		return
	}

	contentType := "image/jpeg"
	if ext == ".png" {
		contentType = "image/png"
	}

	key := fmt.Sprintf("cv-photos/%d/%d%s", userID, time.Now().UnixNano(), ext)
	if err := models.UploadToS3WithContentType(key, data, contentType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload photo", "details": err.Error()})
		return
	}

	photoURL := models.S3ObjectURL(key)

	if err := models.UpsertCVProfilePhotoURL(database.DB, userID, photoURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profile_photo_url": models.ClientAccessiblePhotoURL(photoURL)})
}

// DownloadCVPDF generates and streams a branded PDF for the authenticated user.
func DownloadCVPDF(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	downloadCVForUser(c, userID)
}

// ---------- Admin handlers ----------

// ListCVsByAdmin returns CV summaries with optional ?skill= and ?qualification= filters.
func ListCVsByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	summaries, err := models.ListCVProfiles(database.DB, parseCVSearchOptions(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CV list"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cvs": summaries, "count": len(summaries)})
}

// SearchCVs lets any authenticated user search the talent directory by skill or qualification.
func SearchCVs(c *gin.Context) {
	_, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	skill := strings.TrimSpace(c.Query("skill"))
	qualification := strings.TrimSpace(c.Query("qualification"))
	queryText := strings.TrimSpace(c.Query("q"))

	if skill == "" && qualification == "" && queryText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide at least one filter: skill, qualification, or q"})
		return
	}

	results, err := models.ListCVProfiles(database.DB, parseCVSearchOptions(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results, "count": len(results)})
}

// GetCVByAdmin returns a specific user's full CV data.
func GetCVByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	targetID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	profile, err := models.GetCVProfileByUserID(database.DB, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CV"})
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CV not found"})
		return
	}
	enrichCVPhotoURL(profile)
	c.JSON(http.StatusOK, gin.H{"cv": profile})
}

// DownloadCVByAdmin generates and streams the PDF for any user.
func DownloadCVByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	targetID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	downloadCVForUser(c, targetID)
}

// ---------- Shared helper ----------

func downloadCVForUser(c *gin.Context, userID int) {
	profile, err := models.GetCVProfileByUserID(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load CV"})
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CV not found. Please fill in your CV details first."})
		return
	}

	pdfBytes, err := cvPDFService.GeneratePDF(profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}

	firstName := strings.TrimSpace(profile.FirstName)
	lastName := strings.TrimSpace(profile.LastName)
	filename := "CV"
	if firstName != "" || lastName != "" {
		filename = fmt.Sprintf("CV_%s_%s", firstName, lastName)
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, filename))
	c.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func parseCVSearchOptions(c *gin.Context) models.CVSearchOptions {
	match := strings.ToLower(strings.TrimSpace(c.DefaultQuery("match", "all")))
	if match != models.CVSearchMatchAny {
		match = "all"
	}

	return models.CVSearchOptions{
		Skill:         strings.TrimSpace(c.Query("skill")),
		Qualification: strings.TrimSpace(c.Query("qualification")),
		Query:         strings.TrimSpace(c.Query("q")),
		Match:         match,
	}
}

func enrichCVPhotoURL(profile *models.CVProfile) {
	if profile == nil {
		return
	}
	profile.ProfilePhotoURL = models.ClientAccessiblePhotoURL(profile.ProfilePhotoURL)
}
