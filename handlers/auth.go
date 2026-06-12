package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/middleware"
	"solusphere_backend/models"
	"solusphere_backend/services"

	"github.com/gin-gonic/gin"
)

type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type updatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

func Register(c *gin.Context) {
	registerWithProvider(c, "local", "User registered successfully. Log in and register your face to complete setup.")
}

func Outlook365Signup(c *gin.Context) {
	registerWithProvider(c, "outlook365", "Outlook 365 account registered successfully. Log in and register your face to complete setup.")
}

func registerWithProvider(c *gin.Context, provider, successMessage string) {
	log.Println("========================================")
	log.Printf("REGISTER HANDLER CALLED provider=%s", provider)
	log.Println("========================================")

	if err := database.EnsureDB(); err != nil {
		log.Printf("EnsureDB failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Registration JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)
	req.PhoneNumber = strings.TrimSpace(req.PhoneNumber)

	if _, err := models.GetUserByEmail(database.DB, req.Email); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
		return
	}

	if _, err := models.GetUserByUsername(database.DB, req.Username); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PhoneNumber:  req.PhoneNumber,
		Password:     req.Password,
		AuthProvider: provider,
		Role:         "user",
	}

	if err := user.HashPassword(); err != nil {
		log.Printf("Password hashing error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	if err := models.CreateUser(database.DB, user); err != nil {
		log.Printf("Database error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user", "details": err.Error()})
		return
	}

	_, err := database.DB.Exec(
		"INSERT INTO user_faces (user_id, image_url, status) VALUES (?, ?, ?)",
		user.ID, "", false,
	)
	if err != nil {
		log.Printf("Warning: failed to create user face profile: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       successMessage,
		"face_required": true,
		"next_step":     "POST /api/face/register",
		"user":          userResponse(user, false, nil),
	})
}

func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Login JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	log.Printf("Login attempt for email: %s", email)

	user, err := models.GetUserByEmail(database.DB, email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if err := user.CheckPassword(req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	faceStatus, imageURL, err := models.GetUserFaceRegistrationStatus(database.DB, user.ID)
	if err != nil {
		log.Printf("Face status lookup error for user %d: %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check face registration status"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		log.Printf("Token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	var imageURLValue interface{}
	if imageURL != "" {
		imageURLValue = imageURL
	}

	nextStep := ""
	if !faceStatus {
		nextStep = "POST /api/face/register"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Login successful",
		"token":         token,
		"face_required": !faceStatus,
		"next_step":     nextStep,
		"user":          userResponse(user, faceStatus, imageURLValue),
	})
}

func ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	user, err := models.GetUserByEmail(database.DB, email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "If this account can receive reset messages, a code has been sent."})
		return
	}

	code, err := generateResetCode()
	if err != nil {
		log.Printf("Password reset code generation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create password reset code"})
		return
	}

	codeHash := hashResetCode(user.ID, code)
	if err := models.ExpireUserPasswordResetCodes(database.DB, user.ID); err != nil {
		log.Printf("Failed to expire previous reset codes for user %d: %v", user.ID, err)
	}
	if err := models.CreatePasswordResetCode(database.DB, user.ID, codeHash, time.Now().Add(15*time.Minute)); err != nil {
		log.Printf("Failed to save password reset code for user %d: %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save password reset code"})
		return
	}

	message := fmt.Sprintf("Your Solusphere password reset code is %s. It expires in 15 minutes.", code)
	channels, deliveryErrors := deliverResetCode(user, message)
	if len(channels) == 0 {
		log.Printf("Password reset delivery failed for user %d: %s", user.ID, strings.Join(deliveryErrors, "; "))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Password reset delivery is not configured",
			"details": deliveryErrors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Password reset code sent.",
		"channels": channels,
		"expires":  "15 minutes",
	})
}

func ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	user, err := models.GetUserByEmail(database.DB, email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset code"})
		return
	}

	codeHash := hashResetCode(user.ID, strings.TrimSpace(req.Code))
	resetCodeID, err := models.GetValidPasswordResetCodeID(database.DB, user.ID, codeHash, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset code"})
		return
	}

	if err := models.UpdateUserPassword(database.DB, user.ID, req.NewPassword); err != nil {
		log.Printf("Failed to update password for user %d: %v", user.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}
	if err := models.MarkPasswordResetCodeUsed(database.DB, resetCodeID); err != nil {
		log.Printf("Failed to mark reset code as used: %v", err)
	}
	if err := models.ExpireUserPasswordResetCodes(database.DB, user.ID); err != nil {
		log.Printf("Failed to expire reset codes for user %d: %v", user.ID, err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func UpdatePassword(c *gin.Context) {
	userID := c.GetInt("userID")

	var req updatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := models.GetUserByID(database.DB, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := user.CheckPassword(req.CurrentPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	if err := models.UpdateUserPassword(database.DB, userID, req.NewPassword); err != nil {
		log.Printf("Failed to update password for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func Profile(c *gin.Context) {
	userID := c.GetInt("userID")

	user, err := models.GetUserByID(database.DB, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	face, err := models.GetUserFaceByUserID(database.DB, userID)
	if err != nil || face == nil {
		c.JSON(http.StatusOK, gin.H{
			"face_required": true,
			"next_step":     "POST /api/face/register",
			"user":          userResponse(user, false, nil),
		})
		return
	}

	faceStatus := face.IsRegistered()
	var imageURLValue interface{}
	if faceStatus && face.ImageURL != "" {
		imageURLValue = face.ImageURL
	}

	nextStep := ""
	if !faceStatus {
		nextStep = "POST /api/face/register"
	}

	c.JSON(http.StatusOK, gin.H{
		"face_required": !faceStatus,
		"next_step":     nextStep,
		"user":          userResponse(user, faceStatus, imageURLValue),
	})
}

func RefreshToken(c *gin.Context) {
	userID := c.GetInt("userID")
	username := c.GetString("username")

	token, err := middleware.GenerateToken(userID, username)
	if err != nil {
		log.Printf("Token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
		"token":   token,
	})
}

func deliverResetCode(user *models.User, message string) ([]string, []string) {
	channels := []string{}
	errors := []string{}

	if strings.TrimSpace(user.PhoneNumber) != "" {
		if err := models.PublishSMS(user.PhoneNumber, message); err != nil {
			errors = append(errors, "sms: "+err.Error())
		} else {
			channels = append(channels, "sms")
		}
	}

	if services.IsMailConfigured() {
		if err := services.SendMail(user.Email, "Solusphere password reset code", message); err != nil {
			errors = append(errors, "email: "+err.Error())
		} else {
			channels = append(channels, "email")
		}
	}

	if strings.TrimSpace(user.PhoneNumber) == "" {
		errors = append(errors, "sms: user phone number is not registered")
	}
	if !services.IsMailConfigured() {
		errors = append(errors, "email: SMTP is not configured")
	}

	return channels, errors
}

func generateResetCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashResetCode(userID int, code string) string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "solusphere-password-reset-development-secret"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d:%s", userID, strings.TrimSpace(code))))
	return hex.EncodeToString(mac.Sum(nil))
}

func userResponse(user *models.User, faceStatus bool, imageURL interface{}) gin.H {
	return gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"phone_number":  user.PhoneNumber,
		"auth_provider": user.AuthProvider,
		"role":          user.Role,
		"face_status":   faceStatus,
		"image_url":     imageURL,
	}
}
