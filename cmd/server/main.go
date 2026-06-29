package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"solusphere_backend/database"
	"solusphere_backend/handlers"
	"solusphere_backend/internal/ai"
	"solusphere_backend/internal/secrets"
	"solusphere_backend/middleware"
	"solusphere_backend/models"
	"solusphere_backend/services"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type AppSecrets struct {
	JWTSecret     string
	OpenAIAPIKey  string
	OpenAIModel   string
	DBUser        string
	DBPassword    string
	DBHost        string
	DBPort        string
	DBName        string
	AWSAccessKey  string
	AWSSecretKey  string
	AWSRegion     string
	AWSBucketName string
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFrom      string
}

func main() {
	// Load .env when present (local development). Silently ignored in production
	// where env vars are injected by the container runtime.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}

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
			OpenAIAPIKey:  getStringValue(secretData, "OPENAI_API_KEY", ""),
			OpenAIModel:   getStringValue(secretData, "OPENAI_MODEL", "gpt-4.1-mini"),
			AWSRegion:     getStringValue(secretData, "AWS_REGION", "eu-west-1"),
			AWSBucketName: getStringValue(secretData, "AWS_BUCKET_NAME", "innovationform"),
			SMTPHost:      getStringValue(secretData, "SMTP_HOST", ""),
			SMTPPort:      getStringValue(secretData, "SMTP_PORT", ""),
			SMTPUsername:  getStringValue(secretData, "SMTP_USERNAME", ""),
			SMTPPassword:  getStringValue(secretData, "SMTP_PASSWORD", ""),
			SMTPFrom:      getStringValue(secretData, "SMTP_FROM", ""),
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
			OpenAIAPIKey:  appSecretsRaw.OpenAIAPIKey,
			OpenAIModel:   appSecretsRaw.OpenAIModel,
			DBUser:        appSecretsRaw.DBUser,
			DBPassword:    appSecretsRaw.DBPassword,
			DBHost:        appSecretsRaw.DBHost,
			DBPort:        getDefault(appSecretsRaw.DBPort, "3306"),
			DBName:        appSecretsRaw.DBName,
			AWSAccessKey:  appSecretsRaw.AWSAccessKey,
			AWSSecretKey:  appSecretsRaw.AWSSecretKey,
			AWSRegion:     appSecretsRaw.AWSRegion,
			AWSBucketName: appSecretsRaw.AWSBucketName,
			SMTPHost:      appSecretsRaw.SMTPHost,
			SMTPPort:      appSecretsRaw.SMTPPort,
			SMTPUsername:  appSecretsRaw.SMTPUsername,
			SMTPPassword:  appSecretsRaw.SMTPPassword,
			SMTPFrom:      appSecretsRaw.SMTPFrom,
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
			OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
			OpenAIModel:   getEnv("OPENAI_MODEL", "gpt-4.1-mini"),
			AWSRegion:     getEnv("AWS_REGION", "eu-west-1"),
			AWSBucketName: getEnv("AWS_BUCKET_NAME", "innovationform"),
			AWSAccessKey:  getEnv("AWS_ACCESS_KEY_ID", ""),
			AWSSecretKey:  getEnv("AWS_SECRET_ACCESS_KEY", ""),
			SMTPHost:      getEnv("SMTP_HOST", ""),
			SMTPPort:      getEnv("SMTP_PORT", ""),
			SMTPUsername:  getEnv("SMTP_USERNAME", ""),
			SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
			SMTPFrom:      getEnv("SMTP_FROM", ""),
		}
	}

	// Debug: Print loaded credentials
	log.Println("=== CREDENTIALS LOADED ===")
	log.Printf("DB_HOST: %s", appSecrets.DBHost)
	log.Printf("DB_USER: %s", appSecrets.DBUser)
	log.Printf("DB_NAME: %s", appSecrets.DBName)
	log.Printf("AWS_REGION: %s", appSecrets.AWSRegion)
	log.Printf("AWS_BUCKET_NAME: %s", appSecrets.AWSBucketName)
	log.Printf("AWS_ACCESS_KEY_ID: %s", configuredLabel(appSecrets.AWSAccessKey))
	log.Printf("AWS_SECRET_ACCESS_KEY: %s", configuredLabel(appSecrets.AWSSecretKey))
	log.Println("==========================")

	// Validate required secrets
	if appSecrets.JWTSecret == "" || appSecrets.JWTSecret == "default-jwt-secret" {
		log.Println(" Warning: JWT_SECRET is not set properly")
	}
	if appSecrets.OpenAIAPIKey == "" {
		log.Println(" Warning: OPENAI_API_KEY is not set")
	}
	if appSecrets.DBPassword == "" {
		log.Println(" Warning: DB_PASSWORD is not set")
	}

	appSecrets.OpenAIModel = ai.NormalizeOpenAIModel(appSecrets.OpenAIModel)

	// Set environment variables from secrets
	os.Setenv("JWT_SECRET", appSecrets.JWTSecret)
	os.Setenv("OPENAI_API_KEY", appSecrets.OpenAIAPIKey)
	os.Setenv("OPENAI_MODEL", appSecrets.OpenAIModel)
	os.Setenv("DB_USER", appSecrets.DBUser)
	os.Setenv("DB_PASSWORD", appSecrets.DBPassword)
	os.Setenv("DB_HOST", appSecrets.DBHost)
	os.Setenv("DB_PORT", appSecrets.DBPort)
	os.Setenv("DB_NAME", appSecrets.DBName)
	os.Setenv("AWS_REGION", appSecrets.AWSRegion)
	os.Setenv("AWS_BUCKET_NAME", appSecrets.AWSBucketName)
	os.Setenv("AWS_ACCESS_KEY_ID", appSecrets.AWSAccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", appSecrets.AWSSecretKey)
	os.Setenv("SMTP_HOST", appSecrets.SMTPHost)
	os.Setenv("SMTP_PORT", appSecrets.SMTPPort)
	os.Setenv("SMTP_USERNAME", appSecrets.SMTPUsername)
	os.Setenv("SMTP_PASSWORD", appSecrets.SMTPPassword)
	os.Setenv("SMTP_FROM", appSecrets.SMTPFrom)

	// Initialize MySQL database
	log.Printf(" Connecting to MySQL database at %s:%s", appSecrets.DBHost, appSecrets.DBPort)
	database.InitDBWithPort(
		appSecrets.DBUser,
		appSecrets.DBPassword,
		appSecrets.DBHost,
		appSecrets.DBPort,
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

	if err := database.EnsureChatAndCallSchema(database.DB); err != nil {
		log.Fatalf(" Failed to ensure chat/call schema: %v", err)
	}
	log.Println(" Chat/call schema verified")

	// CRITICAL: Verify database connection is still alive after migrations
	log.Println(" Verifying database connection after migrations...")
	if err := database.DB.Ping(); err != nil {
		log.Printf(" Database connection lost after migrations: %v", err)
		log.Println(" Reconnecting to database...")
		database.InitDBWithPort(
			appSecrets.DBUser,
			appSecrets.DBPassword,
			appSecrets.DBHost,
			appSecrets.DBPort,
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

	// Initialize AWS services. In development this can use explicit keys from
	// .env; on ECS it should use the task role through the AWS default chain.
	if appSecrets.AWSRegion != "" && appSecrets.AWSBucketName != "" {
		log.Println(" Initializing AWS services...")
		if err := models.InitAWSWithSecrets(
			appSecrets.AWSAccessKey,
			appSecrets.AWSSecretKey,
			appSecrets.AWSRegion,
			appSecrets.AWSBucketName,
		); err != nil {
			log.Printf(" Warning: Failed to initialize AWS clients: %v", err)
		} else {
			log.Println(" AWS S3, SNS, and Rekognition clients initialized successfully")
		}
	} else {
		log.Println(" AWS region or bucket not provided, skipping AWS services initialization")
	}

	// Initialize Rekognition service when AWS region and bucket are available.
	var rekogSvc *services.RekognitionService
	if appSecrets.AWSRegion != "" && appSecrets.AWSBucketName != "" {
		log.Println(" Initializing Rekognition service...")
		rekogSvc = services.NewRekognitionServiceWithCredentials(
			appSecrets.AWSAccessKey,
			appSecrets.AWSSecretKey,
			appSecrets.AWSRegion,
			appSecrets.AWSBucketName,
		)
	} else {
		log.Println(" Rekognition region or bucket not provided; face recognition endpoints will return 503")
	}

	// Initialize OpenAI service
	if appSecrets.OpenAIAPIKey != "" {
		log.Println(" Initializing OpenAI service...")
		services.InitOpenAIWithKey(appSecrets.OpenAIAPIKey, appSecrets.OpenAIModel)
		log.Printf(" OpenAI model: %s", services.GetOpenAIModel())
	} else {
		log.Println(" OpenAI API key not provided; AI endpoints will return fallback responses or 503")
	}

	// Initialize OpenAI service for PDF processing
	var openAIService *services.OpenAIService
	if appSecrets.OpenAIAPIKey != "" {
		openAIService, err = services.NewOpenAIService(appSecrets.OpenAIAPIKey, appSecrets.OpenAIModel)
		if err != nil {
			log.Printf(" Warning: Failed to initialize OpenAI service: %v", err)
		} else {
			defer openAIService.Close()
			log.Println(" OpenAI service initialized successfully")
		}
	}

	// Initialize PDF Processor
	pdfProcessor := services.NewPDFProcessor(openAIService)

	// Initialize Upload Service
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	uploadService := services.NewUploadService(uploadDir)

	// Initialize BPO Analysis Handler
	bpoHandler := handlers.NewBPOAnalysisHandler(database.DB, pdfProcessor, uploadService)
	go bpoHandler.ResumePendingAnalyses()

	// Create Gin router
	r := gin.Default()
	r.Static("/uploads", uploadDir)

	// CORS configuration
	r.Use(cors.New(cors.Config{

		AllowOrigins: []string{
			"http://localhost:5173",
			"http://localhost:3000",
			"http://localhost:2080",
			"http://3.250.102.248:2080",
			"https://d25x8zzf939iqa.cloudfront.net",
			"http://solusphere-frontend.s3-website-us-east-1.amazonaws.com",
			"http://solusphere-ui.s3-website-eu-west-1.amazonaws.com",        // Add this
			"https://solusphere-frontend.s3-website-us-east-1.amazonaws.com", // Also add HTTPS version
			"https://d22snf4es6f4ui.cloudfront.net",
		},
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

	// Swagger API documentation
	r.GET("/", handlers.SwaggerRoot)
	r.HEAD("/", handlers.SwaggerRoot)
	r.GET("/swagger", handlers.SwaggerRoot)
	r.HEAD("/swagger", handlers.SwaggerRoot)
	r.GET("/swagger/", handlers.SwaggerRoot)
	r.HEAD("/swagger/", handlers.SwaggerRoot)
	r.GET("/swagger/index.html", handlers.SwaggerIndex)
	r.GET("/swagger/doc.json", handlers.SwaggerSpec)

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

	// ============================================
	// ROUTES REGISTRATION
	// ============================================

	// Public routes - Authentication (no auth required)
	public := r.Group("/api/auth")
	{
		public.POST("/register", handlers.Register)
		public.POST("/outlook365/signup", handlers.Outlook365Signup)
		public.POST("/login", handlers.Login)
		public.POST("/forgot-password", handlers.ForgotPassword)
		public.POST("/reset-password", handlers.ResetPassword)
		public.POST("/face-login", handlers.FaceLogin(rekogSvc))
	}

	// Public face recognition endpoint (for login)
	r.POST("/api/upload-face", handlers.FaceLogin(rekogSvc))

	// Protected routes (require authentication)
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	protected.Use(middleware.RequireCompletedFaceRegistration())
	{
		// User routes
		protected.GET("/profile", handlers.Profile)
		protected.PATCH("/auth/password", handlers.UpdatePassword)
		protected.GET("/users", handlers.ListChatUsers)
		protected.POST("/presence/heartbeat", handlers.TouchPresence)
		protected.GET("/presence", handlers.ListUserPresence)
		protected.GET("/consent", handlers.GetUserConsents)
		protected.POST("/consent", handlers.SignUserConsent)

		// Face registration - requires authentication
		protected.POST("/face/register", handlers.RegisterFace(rekogSvc))
		protected.PUT("/face/update", handlers.UpdateFace(rekogSvc))
		protected.DELETE("/face/delete", handlers.DeleteFace(rekogSvc))

		// Helpdesk routes
		protected.POST("/helpdesk", handlers.SubmitTicketHandler)
		protected.POST("/helpdesk-chat", handlers.HelpdeskChatHandler)

		// Event chat routes
		protected.GET("/events", handlers.ListEvents)
		protected.POST("/events/:event_id/join", handlers.JoinEvent)
		protected.GET("/events/:event_id/comments", handlers.ListEventMessages)
		protected.POST("/events/:event_id/comments", handlers.SendEventMessage)
		protected.GET("/events/:event_id/messages", handlers.ListEventMessages)
		protected.POST("/events/:event_id/messages", handlers.SendEventMessage)

		// Direct user chat routes
		protected.GET("/chats", handlers.ListDirectConversations)
		protected.GET("/chats/calls/incoming", handlers.ListIncomingCalls)
		protected.GET("/chats/calls/:call_id", handlers.GetCall)
		protected.POST("/chats/calls/:call_id/accept", handlers.AcceptCall)
		protected.POST("/chats/calls/:call_id/reject", handlers.RejectCall)
		protected.POST("/chats/calls/:call_id/end", handlers.EndCall)
		protected.POST("/chats/calls/:call_id/candidates", handlers.AddCallCandidate)
		protected.GET("/chats/calls/:call_id/candidates", handlers.ListCallCandidates)
		protected.GET("/chats/:user_id/messages", handlers.ListDirectMessages)
		protected.POST("/chats/:user_id/messages", handlers.SendDirectMessage)
		protected.POST("/chats/:user_id/calls", handlers.StartCall)

		// Admin event chat routes
		admin := protected.Group("/admin")
		{
			admin.POST("/bootstrap", handlers.BootstrapAdmin)
			admin.GET("/users", handlers.ListUsersByAdmin)
			admin.POST("/users", handlers.CreateUserByAdmin)
			admin.GET("/users/:user_id", handlers.GetUserByAdmin)
			admin.PATCH("/users/:user_id", handlers.UpdateUserByAdmin)
			admin.DELETE("/users/:user_id", handlers.DeleteUserByAdmin)
			admin.PATCH("/users/:user_id/role", handlers.UpdateUserRole)
			admin.POST("/events", handlers.CreateEvent)
			admin.PATCH("/events/:event_id", handlers.UpdateEventByAdmin)
			admin.DELETE("/events/:event_id", handlers.DeleteEventByAdmin)
			admin.GET("/events/:event_id/comments", handlers.ListEventMessages)
			admin.POST("/events/:event_id/comments", handlers.SendEventMessage)
			admin.GET("/events/:event_id/messages", handlers.ListEventMessages)
			admin.POST("/events/:event_id/messages", handlers.SendEventMessage)
			admin.GET("/helpdesk", handlers.ListHelpdeskTicketsByAdmin)
			admin.GET("/helpdesk/:ticket_id", handlers.GetHelpdeskTicketByAdmin)
			admin.PATCH("/helpdesk/:ticket_id", handlers.UpdateHelpdeskTicketByAdmin)
			admin.DELETE("/helpdesk/:ticket_id", handlers.DeleteHelpdeskTicketByAdmin)
			admin.GET("/login-audit", handlers.ListLoginAuditLogsByAdmin)
		}

		// Chatbot routes
		protected.POST("/chatbot", handlers.ChatbotHandler())
		protected.POST("/chatbot/report", handlers.SIAReportHandler())

		// BPO Analysis routes
		bpo := protected.Group("/bpo")
		{
			bpo.POST("/analyze-pdf", bpoHandler.UploadAndAnalyzePDF)
			bpo.GET("/analysis/:id", bpoHandler.GetAnalysisResult)
			bpo.POST("/analysis/:id/reprocess", bpoHandler.ReprocessAnalysis)
			bpo.GET("/analyses", bpoHandler.ListAnalyses)
			bpo.DELETE("/analysis/:id", bpoHandler.DeleteAnalysis)
		}

		// CV Builder routes
		cv := protected.Group("/cv")
		{
			cv.GET("", handlers.GetCV)
			cv.POST("", handlers.CreateOrUpdateCV)
			cv.PATCH("", handlers.CreateOrUpdateCV)
			cv.DELETE("", handlers.DeleteCV)
			cv.POST("/photo", handlers.UploadCVPhoto)
			cv.POST("/import", handlers.ImportCVFromDocument)
			cv.GET("/download", handlers.DownloadCVPDF)
			cv.GET("/search", handlers.SearchCVs)
		}

		// Admin CV routes
		admin.GET("/cvs", handlers.ListCVsByAdmin)
		admin.GET("/cvs/:user_id", handlers.GetCVByAdmin)
		admin.GET("/cvs/:user_id/download", handlers.DownloadCVByAdmin)

		// File upload routes
		protected.POST("/upload", handlers.UploadHandler)
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		dbStatus := "healthy"
		if err := database.DB.Ping(); err != nil {
			dbStatus = "unhealthy: " + err.Error()
		}

		awsStatus := "not_configured"
		if appSecrets.AWSRegion != "" && appSecrets.AWSBucketName != "" {
			awsStatus = "configured"
		}
		openAIStatus := "not_configured"
		if services.IsOpenAIInitialized() {
			openAIStatus = "configured"
		}

		c.JSON(200, gin.H{
			"message":        "Server is running",
			"timestamp":      time.Now().Format(time.RFC3339),
			"database":       dbStatus,
			"database_type":  "MySQL",
			"database_host":  appSecrets.DBHost,
			"database_name":  appSecrets.DBName,
			"aws_status":     awsStatus,
			"openai_status":  openAIStatus,
			"openai_model":   services.GetOpenAIModel(),
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
	log.Println("Available endpoints:")
	log.Println("  GET  /swagger/index.html")
	log.Println("  GET  /swagger/doc.json")
	log.Println("  POST /api/auth/register")
	log.Println("  POST /api/auth/outlook365/signup")
	log.Println("  POST /api/auth/login")
	log.Println("  POST /api/auth/forgot-password")
	log.Println("  POST /api/auth/reset-password")
	log.Println("  POST /api/auth/face-login  ✅ Face recognition login")
	log.Println("  POST /api/upload-face      ✅ Alias for face-login")
	log.Println("  POST /api/face/register    ✅ Register new face (requires auth)")
	log.Println("  PUT  /api/face/update      ✅ Update face (requires auth)")
	log.Println("  DELETE /api/face/delete    ✅ Delete face (requires auth)")
	log.Println("  GET  /api/consent (requires auth)")
	log.Println("  POST /api/consent (requires auth)")
	log.Println("  PATCH /api/auth/password (requires auth)")
	log.Println("  POST /api/helpdesk (requires auth)")
	log.Println("  GET  /api/events (requires auth)")
	log.Println("  POST /api/events/:event_id/join (requires auth)")
	log.Println("  GET  /api/events/:event_id/comments (requires auth)")
	log.Println("  POST /api/events/:event_id/comments (requires auth)")
	log.Println("  GET  /api/events/:event_id/messages (requires auth)")
	log.Println("  POST /api/events/:event_id/messages (requires auth)")
	log.Println("  GET  /api/users (requires auth)")
	log.Println("  GET  /api/chats (requires auth)")
	log.Println("  GET  /api/chats/:user_id/messages (requires auth)")
	log.Println("  POST /api/chats/:user_id/messages (requires auth)")
	log.Println("  POST /api/admin/bootstrap (requires auth, first admin only)")
	log.Println("  GET  /api/admin/users (admin)")
	log.Println("  POST /api/admin/users (admin)")
	log.Println("  GET  /api/admin/users/:user_id (admin)")
	log.Println("  PATCH /api/admin/users/:user_id (admin)")
	log.Println("  DELETE /api/admin/users/:user_id (admin)")
	log.Println("  PATCH /api/admin/users/:user_id/role (admin)")
	log.Println("  POST /api/admin/events (admin)")
	log.Println("  PATCH /api/admin/events/:event_id (admin)")
	log.Println("  DELETE /api/admin/events/:event_id (admin)")
	log.Println("  GET  /api/admin/helpdesk (admin)")
	log.Println("  GET  /api/admin/helpdesk/:ticket_id (admin)")
	log.Println("  PATCH /api/admin/helpdesk/:ticket_id (admin)")
	log.Println("  DELETE /api/admin/helpdesk/:ticket_id (admin)")
	log.Println("  GET  /api/admin/login-audit (admin)")
	log.Println("  POST /api/chatbot (requires auth)")
	log.Println("  POST /api/chatbot/report (requires auth)")
	log.Println("  POST /api/bpo/analyze-pdf (requires auth)")
	log.Println("  GET  /api/cv (requires auth)")
	log.Println("  POST /api/cv (requires auth)")
	log.Println("  PATCH /api/cv (requires auth)")
	log.Println("  DELETE /api/cv (requires auth)")
	log.Println("  POST /api/cv/photo (requires auth)")
	log.Println("  POST /api/cv/import (requires auth)")
	log.Println("  GET  /api/cv/download (requires auth)")
	log.Println("  GET  /api/cv/search?skill=&qualification= (requires auth)")
	log.Println("  GET  /api/admin/cvs (admin)")
	log.Println("  GET  /api/admin/cvs/:user_id (admin)")
	log.Println("  GET  /api/admin/cvs/:user_id/download (admin)")
	log.Println("  GET  /health")
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

func getDefault(value, defaultValue string) string {
	if value != "" {
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

func configuredLabel(value string) string {
	if value == "" {
		return "(not set)"
	}
	return "(set)"
}
