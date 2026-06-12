package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type createEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

type eventMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

type updateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type adminCreateUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password" binding:"required,min=6"`
	Role        string `json:"role"`
}

func CreateEvent(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !requireAdmin(c, userID) {
		return
	}

	var req createEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	imageURL := normalizeEventImageURL(req.ImageURL)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event title is required"})
		return
	}
	if len(title) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event title must be 255 characters or fewer"})
		return
	}

	event, err := models.CreateEvent(database.DB, title, description, imageURL, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"event": event})
}

func ListEvents(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	isAdmin, err := models.IsAdmin(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user role"})
		return
	}

	events, err := models.ListEvents(database.DB, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}

func JoinEvent(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	eventID, ok := parseEventID(c)
	if !ok {
		return
	}

	event, err := models.GetEventByID(database.DB, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load event"})
		return
	}

	if event.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"error": "Event is closed"})
		return
	}

	if err := models.JoinEvent(database.DB, eventID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined event successfully", "event": event})
}

func ListEventMessages(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	eventID, ok := parseEventID(c)
	if !ok {
		return
	}

	isAdmin, err := models.IsAdmin(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user role"})
		return
	}

	allowed, err := models.CanAccessEventChat(database.DB, eventID, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check event access"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Join this event before viewing chat"})
		return
	}

	limit := 50
	if rawLimit := c.Query("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsedLimit
		}
	}

	messages, err := models.ListEventChatMessages(database.DB, eventID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load event messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"comments": messages, "messages": messages})
}

func SendEventMessage(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	eventID, ok := parseEventID(c)
	if !ok {
		return
	}

	event, err := models.GetEventByID(database.DB, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load event"})
		return
	}

	if event.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"error": "Event is closed"})
		return
	}

	isAdmin, err := models.IsAdmin(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user role"})
		return
	}

	allowed, err := models.CanAccessEventChat(database.DB, eventID, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check event access"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Join this event before sending messages"})
		return
	}

	var req eventMessageRequest
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

	senderRole := models.RoleUser
	if isAdmin {
		senderRole = models.RoleAdmin
	}

	message, err := models.CreateEventChatMessage(database.DB, eventID, userID, senderRole, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send event message"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"comment": message, "message": message})
}

func UpdateUserRole(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !requireAdmin(c, userID) {
		return
	}

	targetUserID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || targetUserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req updateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	role := strings.TrimSpace(strings.ToLower(req.Role))
	if targetUserID == userID && role != models.RoleAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admins cannot remove their own admin role"})
		return
	}

	if err := models.SetUserRole(database.DB, targetUserID, role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedUser, err := models.GetUserByID(database.DB, targetUserID)
	if err != nil || updatedUser == nil {
		c.JSON(http.StatusOK, gin.H{"message": "User role updated", "role": role})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User role updated", "user": adminUserResponse(updatedUser)})
}

func CreateUserByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !requireAdmin(c, userID) {
		return
	}

	var req adminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))
	phoneNumber := strings.TrimSpace(req.PhoneNumber)
	role := strings.TrimSpace(strings.ToLower(req.Role))
	if role == "" {
		role = models.RoleUser
	}
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}
	if role != models.RoleUser && role != models.RoleAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be user or admin"})
		return
	}

	if _, err := models.GetUserByEmail(database.DB, email); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
		return
	}
	if _, err := models.GetUserByUsername(database.DB, username); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	newUser := &models.User{
		Username:     username,
		Email:        email,
		PhoneNumber:  phoneNumber,
		Password:     req.Password,
		AuthProvider: "local",
		Role:         role,
	}
	if err := newUser.HashPassword(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	if err := models.CreateUser(database.DB, newUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	if role != models.RoleUser {
		if err := models.SetUserRole(database.DB, newUser.ID, role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set user role"})
			return
		}
		newUser.Role = role
	}

	_, err := database.DB.Exec(
		"INSERT INTO user_faces (user_id, image_url, status) VALUES (?, ?, ?)",
		newUser.ID, "", false,
	)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{
			"message": "User created, but face profile could not be initialized",
			"user":    adminUserResponse(newUser),
		})
		return
	}

	createdUser, err := models.GetUserByID(database.DB, newUser.ID)
	if err == nil && createdUser != nil {
		newUser = createdUser
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "User created successfully",
		"face_required": true,
		"next_step":     "POST /api/face/register",
		"user":          adminUserResponse(newUser),
	})
}

func BootstrapAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	adminCount, err := models.CountAdmins(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check admin users"})
		return
	}
	if adminCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Admin bootstrap is already complete"})
		return
	}

	if err := models.SetUserRole(database.DB, userID, models.RoleAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bootstrap admin"})
		return
	}

	updatedUser, err := models.GetUserByID(database.DB, userID)
	if err != nil || updatedUser == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Admin bootstrap complete", "role": models.RoleAdmin})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Admin bootstrap complete", "user": adminUserResponse(updatedUser)})
}

func adminUserResponse(user *models.User) gin.H {
	return gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"phone_number":  user.PhoneNumber,
		"auth_provider": user.AuthProvider,
		"role":          user.Role,
		"created_at":    user.CreatedAt,
	}
}

func normalizeEventImageURL(rawURL string) string {
	imageURL := strings.TrimSpace(rawURL)
	if strings.HasPrefix(imageURL, "uploads/") {
		return "/" + imageURL
	}
	return imageURL
}

func requireAdmin(c *gin.Context, userID int) bool {
	isAdmin, err := models.IsAdmin(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user role"})
		return false
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return false
	}
	return true
}

func currentUserID(c *gin.Context) (int, bool) {
	userID := c.GetInt("userID")
	return userID, userID != 0
}

func parseEventID(c *gin.Context) (int, bool) {
	eventID, err := strconv.Atoi(c.Param("event_id"))
	if err != nil || eventID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return 0, false
	}
	return eventID, true
}
