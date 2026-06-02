package handlers

import (
	"log"
	"net/http"

	"solusphere_backend/database"
	"solusphere_backend/middleware"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

// Register handles user registration
func Register(c *gin.Context) {
	log.Println("========================================")
	log.Println("📝 REGISTER HANDLER CALLED")
	log.Println("========================================")

	// Debug database connection
	log.Printf("🔍 Database DB pointer: %p", database.DB)
	log.Printf("🔍 Database status: %s", database.GetDBStatus())

	// Ensure database connection
	if err := database.EnsureDB(); err != nil {
		log.Printf("❌ EnsureDB failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	log.Printf("✅ Database connection verified")

	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Attempting to register user: %s (%s)", req.Username, req.Email)

	// Check if email already exists
	_, err := models.GetUserByEmail(database.DB, req.Email)
	if err == nil {
		log.Printf("Email already exists: %s", req.Email)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
		return
	}

	// Check if username already exists
	_, err = models.GetUserByUsername(database.DB, req.Username)
	if err == nil {
		log.Printf("Username already exists: %s", req.Username)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	// Create new user
	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	}

	// Hash password
	if err := user.HashPassword(); err != nil {
		log.Printf("Password hashing error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Save user to database
	log.Println("Calling models.CreateUser...")
	if err := models.CreateUser(database.DB, user); err != nil {
		log.Printf("Database error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user", "details": err.Error()})
		return
	}

	log.Printf("User created successfully with ID: %d", user.ID)

	// Insert into user_faces table with status = false by default
	_, err = database.DB.Exec(
		"INSERT INTO user_faces (user_id, image_url, status) VALUES (?, ?, ?)",
		user.ID, "", false,
	)

	if err != nil {
		log.Printf("Warning: Failed to create user face profile: %v", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"email":       user.Email,
			"face_status": false,
		},
	})
	log.Println("========================================")
	log.Println("✅ REGISTRATION COMPLETED")
	log.Println("========================================")
}

// Login handles user login using Email
func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Login JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Login attempt for email: %s", req.Email)

	// Get user from database by email
	user, err := models.GetUserByEmail(database.DB, req.Email)
	if err != nil {
		log.Printf("User not found: %s", req.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Check password
	if err := user.CheckPassword(req.Password); err != nil {
		log.Printf("Invalid password for user: %s", req.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		log.Printf("Token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	log.Printf("User logged in successfully: %s (ID: %d)", user.Username, user.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Profile returns user profile (protected route)
func Profile(c *gin.Context) {
	userID := c.GetInt("userID")

	log.Printf("Fetching profile for user ID: %d", userID)

	// Fetch user details
	user, err := models.GetUserByID(database.DB, userID)
	if err != nil {
		log.Printf("User not found: ID %d", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Fetch face record
	face, err := models.GetUserFaceByUserID(database.DB, userID)
	if err != nil {
		// User exists but face not yet captured
		log.Printf("No face record found for user ID %d", userID)
		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":          user.ID,
				"username":    user.Username,
				"email":       user.Email,
				"face_status": false,
				"image_url":   nil,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"email":       user.Email,
			"face_status": face.Status,
			"image_url":   face.ImageURL,
		},
	})
}

// RefreshToken handles token refresh
func RefreshToken(c *gin.Context) {
	userID := c.GetInt("userID")
	username := c.GetString("username")

	log.Printf("Refreshing token for user: %s (ID: %d)", username, userID)

	// Generate new token
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
