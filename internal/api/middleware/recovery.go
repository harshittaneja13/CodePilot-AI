package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// RecoveryMiddleware returns a Gin middleware that recovers from panics,
// logs the stack trace, and returns a 500 JSON error response.
func RecoveryMiddleware(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())

				requestID, _ := c.Get("request_id")

				log.Error().
					Interface("error", err).
					Str("stack", stack).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Interface("request_id", requestID).
					Msg("panic recovered")

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": "internal server error",
				})
			}
		}()

		c.Next()
	}
}
