package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/go-backend-template/internal/delivery/http/response"
)

// RequestDataGuardMiddleware provides basic security headers and request validation
func RequestDataGuardMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Check for suspicious patterns in URL path
		path := strings.ToLower(c.Request.URL.Path)
		suspiciousPatterns := []string{
			"../",
			"..\\",
			"<script",
			"javascript:",
			"vbscript:",
			"onload=",
			"onerror=",
		}

		for _, pattern := range suspiciousPatterns {
			if strings.Contains(path, pattern) {
				response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format", nil)
				return
			}
		}

		// Limit request size (10MB)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10*1024*1024)

		c.Next()
	}
}
