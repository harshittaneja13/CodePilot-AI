package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/codepilot-ai/codepilot-ai/internal/metrics"
)

// MetricsMiddleware records request count and latency per route in the metrics
// registry (exposed at /api/metrics). It uses the route pattern (c.FullPath) rather
// than the raw URL to keep label cardinality bounded.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		metrics.ObserveHTTP(c.Request.Method, path, strconv.Itoa(c.Writer.Status()), time.Since(start).Seconds())
	}
}
