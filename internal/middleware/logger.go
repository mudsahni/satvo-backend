package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID injects an X-Request-ID header into the request and response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger logs each HTTP request with method, path, status, and latency.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		requestID, _ := c.Get("request_id")
		log.Printf("[%s] %s %s %d %s",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			latency,
		)
	}
}

// Recovery recovers from panics and returns a 500 error.
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
