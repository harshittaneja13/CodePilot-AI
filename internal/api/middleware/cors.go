// Package middleware provides HTTP middleware for the CodePilot AI API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns a Gin middleware that adds CORS headers to responses.
// It allows the frontend at localhost:3000 to interact with the API.
func CORSMiddleware() gin.HandlerFunc {
	allowedOrigins := []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		for _, allowed := range allowedOrigins {
			if strings.EqualFold(origin, allowed) {
				c.Header("Access-Control-Allow-Origin", origin)
				break
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
