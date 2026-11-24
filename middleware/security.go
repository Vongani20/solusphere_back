package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds response headers that browsers use to block sniffing and framing.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")
		c.Header("Permissions-Policy", "geolocation=(), payment=(), usb=()")
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			c.Header("Strict-Transport-Security", "max-age=15552000")
		}
		c.Next()
	}
}

// TooManyRequests is the shared response for rate limits.
func TooManyRequests(c *gin.Context) {
	c.Header("Retry-After", "60")
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": "Too many attempts. Wait a few minutes and try again.",
	})
}
