package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestTracingMiddleware generates a unique RequestID for each request and attaches it to the context and response headers.
func RequestTracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if the RequestID header already exists
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a new UUID if not present
			requestID = uuid.New().String()
		}

		// Set the RequestID in the context and response header
		c.Set("RequestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		// Continue to the next middleware/handler
		c.Next()
	}
}
