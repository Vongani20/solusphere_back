package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"solusphere_backend/database"
	"solusphere_backend/handlers"
	"solusphere_backend/internal/secrets"
	"solusphere_backend/middleware"
	"solusphere_backend/models"
	"solusphere_backend/services"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type AppSecrets struct {
	JWTSecret     string
	GeminiAPIKey  string
	DBUser        string
	DBPassword    string
	DBHost        string
	DBPort        string
	DBName        string
	AWSAccessKey  string
	AWSSecretKey  string
	AWSRegion     string
	AWSBucketName string
}

func main() {
	var appSecrets *AppSecrets
	var err error

	// Check if we should use AWS Secrets Manager
	useAWSSecrets := os.Getenv("USE_AWS_SECRETS_MANAGER") == "true"
	useSecretsAgent := os.Getenv("USE_SECRETS_AGENT") == "true"

	log.Println("========================================")
	log.Println(" SOLUSPHERE BACKEND STARTING")
	log.Println("========================================")
	log.Printf("USE_AWS_SECRETS_MANAGER: %v", useAWSSecrets)
	log.Printf("USE_SECRETS_AGENT: %v", useSecretsAgent)

	if useSecretsAgent {
		log.Println(" Initializing Secrets Manager Agent...")
		agent, err := secrets.NewSecretsManagerAgent()
		if err != nil {
			log.Fatalf("Failed to create secrets agent: %v", err)
		}

		secretData, err := agent.GetSecret("solusphere/prod")
		if err != nil {
			log.Fatalf("Failed to get secret from agent: %v", err)
		}

		// Parse secrets from agent
		appSecrets = &AppSecrets{
			DBHost:        getStringValue(secretData, "DB_HOST", "db"),
			DBPort:        getStringValue(secretData, "DB_PORT", "3306"),
			DBUser:        getStringValue(secretData, "DB_USER", "solusphere_users"),
			DBPassword:    getStringValue(secretData, "DB_PASSWORD", "Caswell1234"),
			DBName:        getStringValue(secretData, "DB_NAME", "solusphere"),
			JWTSecret:     getStringValue(secretData, "JWT_SECRET", "default-jwt-secret"),
			GeminiAPIKey:  getStringValue(secretData, "GEMINI_API_KEY", ""),
			AWSRegion:     getStringValue(secretData, "AWS_REGION", "eu-west-1"),
			AWSBucketName: getStringValue(secretData, "AWS_BUCKET_NAME", "innovationform"),
		}
		log.Println(" Successfully retrieved secrets from Secrets Manager Agent")

	} else if useAWSSecrets {
		log.Println(" Initializing AWS Secrets Manager...")
		secretName := os.Getenv("AWS_SECRET_NAME")
		if secretName == "" {
			secretName = "solusphere/prod"
		}
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "eu-west-1"
		}

		secretsManager, err := secrets.GetInstance(secretName, region)
		if err != nil {
			log.Fatalf(" Failed to initialize secrets manager: %v", err)
		}

		// Get secrets
		ctx := context.Background()
		appSecretsRaw, err := secretsManager.GetSecrets(ctx)
		if err != nil {
			log.Fatalf(" Failed to retrieve secrets: %v", err)
		}
		log.Println(" Successfully retrieved secrets from AWS Secrets Manager")

		// Start background refresh of secrets
		secretsManager.StartRefreshTicker(ctx, 15*time.Minute)
		log.Println(" Secret auto-refresh enabled (every 15 minutes)")

		// Convert to AppSecrets
		appSecrets = &AppSecrets{
			JWTSecret:     appSecretsRaw.JWTSecret,
			GeminiAPIKey:  appSecretsRaw.GeminiAPIKey,
			DBUser:        appSecretsRaw.DBUser,
			DBPassword:    appSecretsRaw.DBPassword,
			DBHost:        appSecretsRaw.DBHost,
			DBPort:        "3306",
			DBName:        appSecretsRaw.DBName,
			AWSAccessKey:  appSecretsRaw.AWSAccessKey,
			AWSSecretKey:  appSecretsRaw.AWSSecretKey,
			AWSRegion:     appSecretsRaw.AWSRegion,
			AWSBucketName: appSecretsRaw.AWSBucketName,
		}

		// Override DB_HOST from environment variable if provided (for Docker)
		if dbHostOverride := os.Getenv("DB_HOST"); dbHostOverride != "" {
			appSecrets.DBHost = dbHostOverride
			log.Printf(" Overriding DB_HOST from secret (%s) with: %s", appSecretsRaw.DBHost, dbHostOverride)
		}
		if dbPortOverride := os.Getenv("DB_PORT"); dbPortOverride != "" {
			appSecrets.DBPort = dbPortOverride
			log.Printf(" Overriding DB_PORT with: %s", dbPortOverride)
		}
		if dbUserOverride := os.Getenv("DB_USER"); dbUserOverride != "" {
			appSecrets.DBUser = dbUserOverride
			log.Printf(" Overriding DB_USER with: %s", dbUserOverride)
		}
		if dbPasswordOverride := os.Getenv("DB_PASSWORD"); dbPasswordOverride != "" {
			appSecrets.DBPassword = dbPasswordOverride
			log.Printf(" Overriding DB_PASSWORD from secret")
		}
		if dbNameOverride := os.Getenv("DB_NAME"); dbNameOverride != "" {
			appSecrets.DBName = dbNameOverride
			log.Printf(" Overriding DB_NAME with: %s", dbNameOverride)
		}

	} else {
		// Use direct environment variables (for local development)
		log.Println(" Using direct environment variables for configuration")
		appSecrets = &AppSecrets{
			DBHost:        getEnv("DB_HOST", "localhost"),
			DBPort:        getEnv("DB_PORT", "3306"),
			DBUser:        getEnv("DB_USER", "root"),
			DBPassword:    getEnv("DB_PASSWORD", ""),
			DBName:        getEnv("DB_NAME", "solusphere"),
			JWTSecret:     getEnv("JWT_SECRET", "default-jwt-secret"),
			GeminiAPIKey:  getEnv("GEMINI_API_KEY", ""),
			AWSRegion:     getEnv("AWS_REGION", "eu-west-1"),
			AWSBucketName: getEnv("AWS_BUCKET_NAME", "innovationform"),
		}
	}

	// Validate required secrets
	if appSecrets.JWTSecret == "" || appSecrets.JWTSecret == "default-jwt-secret" {
		log.Println(" Warning: JWT_SECRET is not set properly")
	}
	if appSecrets.GeminiAPIKey == "" {
		log.Println(" Warning: GEMINI_API_KEY is not set")
	}
	if appSecrets.DBPassword == "" {
		log.Println(" Warning: DB_PASSWORD is not set")
	}

	// Set environment variables from secrets
	os.Setenv("JWT_SECRET", appSecrets.JWTSecret)
	os.Setenv("GEMINI_API_KEY", appSecrets.GeminiAPIKey)
	os.Setenv("DB_USER", appSecrets.DBUser)
	os.Setenv("DB_PASSWORD", appSecrets.DBPassword)
	os.Setenv("DB_HOST", appSecrets.DBHost)
	os.Setenv("DB_NAME", appSecrets.DBName)
	os.Setenv("AWS_REGION", appSecrets.AWSRegion)
	os.Setenv("AWS_BUCKET_NAME", appSecrets.AWSBucketName)

	// Initialize MySQL database
	log.Printf(" Connecting to MySQL database at %s:%s", appSecrets.DBHost, appSecrets.DBPort)
	database.InitDBWithSecrets(
		appSecrets.DBUser,
		appSecrets.DBPassword,
		appSecrets.DBHost,
		appSecrets.DBName,
	)

	// Test database connection
	if err := database.DB.Ping(); err != nil {
		log.Fatalf(" Failed to ping database: %v", err)
	}
	log.Println(" Database connected successfully")

	// Run database migrations
	log.Println(" Running database migrations...")
	if err := database.RunMigrations(database.DB); err != nil {
		log.Printf(" Migration warning: %v", err)
	} else {
		log.Println(" Database migrations completed")
	}

	// CRITICAL: Verify database connection is still alive after migrations
	log.Println(" Verifying database connection after migrations...")
	if err := database.DB.Ping(); err != nil {
		log.Printf(" Database connection lost after migrations: %v", err)
		log.Println(" Reconnecting to database...")
		database.InitDBWithSecrets(
			appSecrets.DBUser,
			appSecrets.DBPassword,
			appSecrets.DBHost,
			appSecrets.DBName,
		)
		if err := database.DB.Ping(); err != nil {
			log.Fatalf(" Failed to reconnect: %v", err)
		}
		log.Println(" Database reconnected successfully")
	} else {
		log.Println(" Database connection is healthy")
	}

	// Log connection status
	log.Printf(" Final DB status: %s", database.GetDBStatus())

	// Initialize AWS services if credentials are provided
	if appSecrets.AWSAccessKey != "" && appSecrets.AWSSecretKey != "" {
		log.Println(" Initializing AWS services...")
		if err := models.InitAWSWithSecrets(
			appSecrets.AWSAccessKey,
			appSecrets.AWSSecretKey,
			appSecrets.AWSRegion,
			appSecrets.AWSBucketName,
		); err != nil {
			log.Printf(" Warning: Failed to initialize AWS clients: %v", err)
		} else {
			log.Println(" AWS S3 and Rekognition clients initialized successfully")
		}
	} else {
		log.Println(" AWS credentials not provided, skipping AWS services initialization")
	}

	// Initialize services
	rekogSvc := services.NewRekognitionService()

	// Initialize Gemini chat service
	if appSecrets.GeminiAPIKey != "" {
		log.Println(" Initializing Gemini AI service...")
		services.InitGeminiWithKey(appSecrets.GeminiAPIKey)
	}

	// Initialize Gemini Service for PDF processing
	var geminiService *services.GeminiService
	if appSecrets.GeminiAPIKey != "" {
		geminiService, err = services.NewGeminiService(appSecrets.GeminiAPIKey)
		if err != nil {
			log.Printf(" Warning: Failed to initialize Gemini service: %v", err)
		} else {
			defer geminiService.Close()
			log.Println(" Gemini service initialized successfully")
		}
	}

	// Initialize PDF Processor
	pdfProcessor := services.NewPDFProcessor(geminiService)

	// Initialize Upload Service
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	uploadService := services.NewUploadService(uploadDir)

	// Initialize BPO Analysis Handler
	bpoHandler := handlers.NewBPOAnalysisHandler(database.DB, pdfProcessor, uploadService)

	// Create Gin router
	r := gin.Default()

	// CORS configuration
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:2080", "http://3.250.102.248:2080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Add request logging middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	})

	// Debug endpoints
	r.GET("/debug/db", func(c *gin.Context) {
		status := database.GetDBStatus()
		c.JSON(200, gin.H{
			"db_status": status,
			"db_ptr":    fmt.Sprintf("%p", database.DB),
			"db_is_nil": database.DB == nil,
			"env_vars": gin.H{
				"DB_HOST": os.Getenv("DB_HOST"),
				"DB_USER": os.Getenv("DB_USER"),
				"DB_NAME": os.Getenv("DB_NAME"),
			},
		})
	})

	r.GET("/debug/ping", func(c *gin.Context) {
		if err := database.DB.Ping(); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "pong"})
	})

	// Public routes
	public := r.Group("/api/auth")
	{
		public.POST("/register", handlers.Register)
		public.POST("/login", handlers.Login)
		public.POST("/face-login", handlers.FaceLogin(rekogSvc))
	}

	// Protected routes
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{
		// User routes
		protected.GET("/profile", handlers.Profile)

		// Helpdesk routes
		protected.POST("/helpdesk", handlers.SubmitTicketHandler)
		protected.POST("/helpdesk-chat", handlers.HelpdeskChatHandler)

		// Chatbot route
		if appSecrets.GeminiAPIKey != "" {
			protected.POST("/chatbot", handlers.ChatbotHandler(appSecrets.GeminiAPIKey))
		}

		// BPO Analysis routes
		bpo := protected.Group("/bpo")
		{
			bpo.POST("/analyze-pdf", bpoHandler.UploadAndAnalyzePDF)
			bpo.GET("/analysis/:id", bpoHandler.GetAnalysisResult)
			bpo.GET("/analyses", bpoHandler.ListAnalyses)
			bpo.DELETE("/analysis/:id", bpoHandler.DeleteAnalysis)
		}

		// File upload routes
		protected.POST("/upload", handlers.UploadHandler)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		dbStatus := "healthy"
		if err := database.DB.Ping(); err != nil {
			dbStatus = "unhealthy: " + err.Error()
		}

		c.JSON(200, gin.H{
			"message":        "Server is running",
			"timestamp":      time.Now().Format(time.RFC3339),
			"database":       dbStatus,
			"database_type":  "MySQL",
			"database_host":  appSecrets.DBHost,
			"database_name":  appSecrets.DBName,
			"aws_status":     "configured",
			"secrets_source": getSecretsSource(useAWSSecrets, useSecretsAgent),
			"secret_name":    os.Getenv("AWS_SECRET_NAME"),
			"port":           2080,
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "2080"
	}

	log.Println("========================================")
	log.Printf(" Server starting on port %s", port)
	log.Printf(" Environment: %s", gin.Mode())
	log.Printf(" Secrets source: %s", getSecretsSource(useAWSSecrets, useSecretsAgent))
	log.Println("========================================")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf(" Failed to start server: %v", err)
	}
}

// Helper function to get environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper function to safely get string value from map
func getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if val, ok := data[key]; ok && val != nil {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return defaultValue
}

// Helper function to get secrets source description
func getSecretsSource(useAWS, useAgent bool) string {
	if useAgent {
		return "secrets-manager-agent"
	}
	if useAWS {
		return "aws-secrets-manager"
	}
	return "environment-variables"
}
