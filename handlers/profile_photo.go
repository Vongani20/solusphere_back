package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

const maxProfilePhotoBytes = 3 << 20 // the UI resizes to 512px first, so real uploads are far smaller

// UploadProfilePhoto POST /api/profile/photo {"image":"<base64 or data URL>"}
// JSON (not multipart) because corporate proxies block multipart file uploads.
func UploadProfilePhoto(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProfilePhotoBytes*2)
	var req struct {
		Image string `json:"image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Image) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose a photo to upload."})
		return
	}
	payload := strings.TrimSpace(req.Image)
	if idx := strings.Index(payload, ","); idx >= 0 && strings.Contains(strings.ToLower(payload[:idx]), "base64") {
		payload = payload[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That photo could not be read."})
		return
	}
	if len(data) > maxProfilePhotoBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Photo must be smaller than 3 MB."})
		return
	}

	contentType := http.DetectContentType(data)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[contentType]
	if ext == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use a JPG, PNG or WEBP photo."})
		return
	}

	key := fmt.Sprintf("profile-photos/%d/photo_%d%s", userID, time.Now().UnixNano(), ext)
	if err := models.UploadToS3WithContentType(key, data, contentType); err != nil {
		log.Printf("profile photo upload for user %d failed: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload photo."})
		return
	}
	stored := models.S3ObjectURL(key)
	if err := models.SetProfilePhotoURL(database.DB, userID, stored); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"image_url": models.ProfilePhotoForClient(stored), "has_profile_photo": true})
}

// DeleteProfilePhoto DELETE /api/profile/photo
func DeleteProfilePhoto(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if err := models.SetProfilePhotoURL(database.DB, userID, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove photo."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "has_profile_photo": false})
}
