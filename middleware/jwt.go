package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

// Claims represents the JWT claims
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.StandardClaims
}

// InitJWTSecret initializes the JWT secret from environment
// This should be called after secrets are loaded from AWS Secrets Manager
func InitJWTSecret() {
	jwtSecretOnce.Do(func() {
		secret := getJWTSecret()
		jwtSecret = []byte(secret)
	})
}

// getJWTSecret retrieves JWT secret from environment
func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// In production, you might want to panic or log a fatal error
		// For development, we'll use a default (but this should never happen in production)
		fmt.Println("WARNING: JWT_SECRET not set, using default (INSECURE - only for development)")
		return "your-secret-key-change-in-production"
	}
	return secret
}

// RefreshJWTSecret refreshes the JWT secret (useful when secrets are rotated)
func RefreshJWTSecret() {
	// Reset the once so we can reinitialize
	jwtSecretOnce = sync.Once{}
	InitJWTSecret()
	fmt.Println("JWT secret refreshed")
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(userID int, username string) (string, error) {
	// Ensure JWT secret is initialized
	InitJWTSecret()

	claims := Claims{
		UserID:   userID,
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "solusphere-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateTokenWithExpiry creates a token with custom expiry
func GenerateTokenWithExpiry(userID int, username string, expiry time.Duration) (string, error) {
	InitJWTSecret()

	claims := Claims{
		UserID:   userID,
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(expiry).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "solusphere-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// AuthMiddleware validates JWT token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ensure JWT secret is initialized
		InitJWTSecret()

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
				"code":  "MISSING_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		// Check if the header has the "Bearer " prefix
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header format must be Bearer {token}",
				"code":  "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse and validate token
		claims, err := validateTokenWithSecret(tokenString, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
				"code":  "INVALID_TOKEN",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("token_expires_at", claims.ExpiresAt)
		
		// Optional: Add token expiration warning if token is about to expire
		if claims.ExpiresAt > 0 {
			expiryTime := time.Unix(claims.ExpiresAt, 0)
			timeLeft := time.Until(expiryTime)
			if timeLeft < 10*time.Minute {
				c.Header("X-Token-Expires-In", timeLeft.String())
			}
		}
		
		c.Next()
	}
}

// validateTokenWithSecret validates a JWT token with given secret
func validateTokenWithSecret(tokenString string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		// Provide more specific error messages
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, fmt.Errorf("malformed token")
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, fmt.Errorf("token expired")
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				return nil, fmt.Errorf("token not active yet")
			}
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// ValidateToken validates a JWT token and returns claims (uses current secret)
func ValidateToken(tokenString string) (*Claims, error) {
	InitJWTSecret()
	return validateTokenWithSecret(tokenString, jwtSecret)
}

// ValidateTokenWithContext validates token and returns claims with user context
func ValidateTokenWithContext(c *gin.Context) (*Claims, error) {
	userID, exists := c.Get("userID")
	if !exists {
		return nil, fmt.Errorf("user not authenticated")
	}
	
	username, _ := c.Get("username")
	
	return &Claims{
		UserID:   userID.(int),
		Username: username.(string),
	}, nil
}

// GetUserIDFromContext retrieves user ID from gin context
func GetUserIDFromContext(c *gin.Context) (int, error) {
	userID, exists := c.Get("userID")
	if !exists {
		return 0, fmt.Errorf("user ID not found in context")
	}
	
	id, ok := userID.(int)
	if !ok {
		return 0, fmt.Errorf("user ID has invalid type")
	}
	
	return id, nil
}

// GetUsernameFromContext retrieves username from gin context
func GetUsernameFromContext(c *gin.Context) (string, error) {
	username, exists := c.Get("username")
	if !exists {
		return "", fmt.Errorf("username not found in context")
	}
	
	name, ok := username.(string)
	if !ok {
		return "", fmt.Errorf("username has invalid type")
	}
	
	return name, nil
}

// OptionalMiddleware is a middleware that doesn't require authentication
// but will process token if present
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		InitJWTSecret()
		
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			
			claims, err := validateTokenWithSecret(tokenString, jwtSecret)
			if err == nil && claims != nil {
				c.Set("userID", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("authenticated", true)
			}
		}
		
		c.Next()
	}
}