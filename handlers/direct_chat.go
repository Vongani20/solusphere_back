package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

const (
	maxChatImageBytes = 10 << 20 // 10 MB
	maxChatVoiceBytes = 15 << 20 // 15 MB
)

type directMessageRequest struct {
	Message string `json:"message"`
}

func ListChatUsers(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	users, err := models.ListChatUsers(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

func ListDirectConversations(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	conversations, err := models.ListDirectConversations(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load conversations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
		"inbox":         models.SummarizeChatInbox(conversations),
	})
}

func ListDirectMessages(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	otherUserID, ok := parseChatUserID(c)
	if !ok {
		return
	}
	if otherUserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot open a direct chat with yourself"})
		return
	}
	if !ensureUserExists(c, otherUserID) {
		return
	}

	limit := 50
	if rawLimit := c.Query("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsedLimit
		}
	}

	messages, err := models.ListDirectMessages(database.DB, userID, otherUserID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
		return
	}

	_ = models.MarkMissedCallsSeenWithPeer(database.DB, userID, otherUserID)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func SendDirectMessage(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	receiverID, ok := parseChatUserID(c)
	if !ok {
		return
	}
	if receiverID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send a direct message to yourself"})
		return
	}
	if !ensureUserExists(c, receiverID) {
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		sendDirectMessageMultipart(c, userID, receiverID)
		return
	}

	sendDirectMessageJSON(c, userID, receiverID)
}

func sendDirectMessageJSON(c *gin.Context, senderID, receiverID int) {
	var req directMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	body := strings.TrimSpace(req.Message)
	if body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message is required"})
		return
	}
	if len(body) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message must be 4000 characters or fewer"})
		return
	}

	message, err := models.CreateDirectMessage(
		database.DB,
		senderID,
		receiverID,
		models.DirectMessageTypeText,
		body,
		"",
		"",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": message})
}

func sendDirectMessageMultipart(c *gin.Context, senderID, receiverID int) {
	messageType := strings.ToLower(strings.TrimSpace(c.PostForm("message_type")))
	caption := strings.TrimSpace(c.PostForm("message"))
	if caption != "" && len(caption) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message must be 4000 characters or fewer"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Attachment file is required"})
		return
	}
	defer file.Close()

	attachmentURL, attachmentMIME, err := uploadChatAttachment(senderID, messageType, file, header.Filename, header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message, err := models.CreateDirectMessage(
		database.DB,
		senderID,
		receiverID,
		messageType,
		caption,
		attachmentURL,
		attachmentMIME,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": message})
}

func uploadChatAttachment(senderID int, messageType string, file io.Reader, filename string, size int64) (string, string, error) {
	switch messageType {
	case models.DirectMessageTypeImage:
		return uploadChatImage(senderID, file, filename, size)
	case models.DirectMessageTypeVoice:
		return uploadChatVoice(senderID, file, filename, size)
	default:
		return "", "", fmt.Errorf("message_type must be image or voice")
	}
}

func uploadChatImage(senderID int, file io.Reader, filename string, size int64) (string, string, error) {
	if size > maxChatImageBytes {
		return "", "", fmt.Errorf("image must be smaller than 10 MB")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		ext = ".jpg"
	case ".png", ".webp":
	default:
		return "", "", fmt.Errorf("only JPG, PNG, and WEBP images are accepted")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read image")
	}

	contentType := "image/jpeg"
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}

	key := fmt.Sprintf("chat-attachments/%d/image_%d%s", senderID, time.Now().UnixNano(), ext)
	if err := models.UploadToS3WithContentType(key, data, contentType); err != nil {
		return "", "", fmt.Errorf("failed to upload image")
	}

	return models.S3ObjectURL(key), contentType, nil
}

func uploadChatVoice(senderID int, file io.Reader, filename string, size int64) (string, string, error) {
	if size > maxChatVoiceBytes {
		return "", "", fmt.Errorf("voice note must be smaller than 15 MB")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	contentType := "audio/webm"
	switch ext {
	case ".webm":
		contentType = "audio/webm"
	case ".mp3":
		contentType = "audio/mpeg"
	case ".m4a":
		contentType = "audio/mp4"
	case ".ogg":
		contentType = "audio/ogg"
	case ".wav":
		contentType = "audio/wav"
	default:
		return "", "", fmt.Errorf("only WEBM, MP3, M4A, OGG, and WAV voice notes are accepted")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read voice note")
	}

	key := fmt.Sprintf("chat-attachments/%d/voice_%d%s", senderID, time.Now().UnixNano(), ext)
	if err := models.UploadToS3WithContentType(key, data, contentType); err != nil {
		return "", "", fmt.Errorf("failed to upload voice note")
	}

	return models.S3ObjectURL(key), contentType, nil
}

func ensureUserExists(c *gin.Context, userID int) bool {
	user, err := models.GetUserByID(database.DB, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user"})
		return false
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return false
	}
	return true
}

func parseChatUserID(c *gin.Context) (int, bool) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return 0, false
	}
	return userID, true
}
