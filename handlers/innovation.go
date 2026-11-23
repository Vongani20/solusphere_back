package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type innovationSubmitRequest struct {
	FullName            string `json:"full_name"`
	Department          string `json:"department"`
	Email               string `json:"email"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Problem             string `json:"problem"`
	Solution            string `json:"solution"`
	DeclarationAccepted bool   `json:"declaration_accepted"`
	Signature           string `json:"signature"`
	PhotoBase64         string `json:"photo"`
	PhotoFilename       string `json:"photo_filename"`
}

type innovationCommitteeRequest struct {
	Reviewer string `json:"reviewer"`
	Status   string `json:"status"`
	Comments string `json:"comments"`
}

func SubmitInnovationIdea(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req innovationSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	photoURL := ""
	if strings.TrimSpace(req.PhotoBase64) != "" {
		url, err := saveInnovationPhoto(userID, req.PhotoBase64, req.PhotoFilename)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		photoURL = url
	}

	idea, err := models.CreateInnovationIdea(database.DB, &models.InnovationIdea{
		UserID:              userID,
		FullName:            req.FullName,
		Department:          req.Department,
		Email:               req.Email,
		Title:               req.Title,
		Description:         req.Description,
		Problem:             req.Problem,
		Solution:            req.Solution,
		PhotoURL:            photoURL,
		DeclarationAccepted: req.DeclarationAccepted,
		Signature:           req.Signature,
		Status:              models.InnovationStatusSubmitted,
	})
	if err != nil {
		log.Printf("SubmitInnovationIdea failed user=%d: %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"idea":    idea,
		"message": "Idea submitted successfully. Thank you!",
	})
}

func ListMyInnovationIdeas(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	ideas, err := models.ListInnovationIdeasByUser(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load innovation ideas"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ideas": ideas})
}

func ListInnovationIdeasByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	ideas, err := models.ListAllInnovationIdeas(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load innovation ideas"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ideas": ideas})
}

func UpdateInnovationIdeaByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	ideaID, err := strconv.Atoi(c.Param("idea_id"))
	if err != nil || ideaID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid idea ID"})
		return
	}

	var req innovationCommitteeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	idea, err := models.UpdateInnovationIdeaCommittee(database.DB, ideaID, req.Reviewer, req.Status, req.Comments)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update idea"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"idea": idea, "message": "Idea updated"})
}

func saveInnovationPhoto(userID int, raw, filename string) (string, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return "", errors.New("photo is empty")
	}
	if idx := strings.Index(payload, ","); idx >= 0 && strings.Contains(strings.ToLower(payload[:idx]), "base64") {
		payload = payload[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil || len(data) == 0 {
		return "", errors.New("invalid photo encoding")
	}

	const maxSize = 5 << 20
	if len(data) > maxSize {
		return "", errors.New("photo must be smaller than 5 MB")
	}

	contentType := http.DetectContentType(data)
	ext := strings.ToLower(filepath.Ext(filename))
	switch contentType {
	case "image/jpeg":
		if ext == "" {
			ext = ".jpg"
		}
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		return "", errors.New("only JPG, PNG, and WEBP photos are accepted")
	}

	key := fmt.Sprintf("innovation-photos/%d/%d%s", userID, time.Now().UnixNano(), ext)
	if err := models.UploadToS3WithContentType(key, data, contentType); err != nil {
		return "", fmt.Errorf("failed to upload photo")
	}
	return models.S3ObjectURL(key), nil
}
