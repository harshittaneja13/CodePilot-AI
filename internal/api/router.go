// Package api wires together the HTTP router with all handlers and middleware.
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/api/handlers"
	"github.com/codepilot-ai/codepilot-ai/internal/api/middleware"
	"github.com/codepilot-ai/codepilot-ai/internal/metrics"
)

// RouterDeps holds all handler dependencies needed to configure routes.
type RouterDeps struct {
	DashboardHandler   *handlers.DashboardHandler
	RepositoryHandler  *handlers.RepositoryHandler
	PullRequestHandler *handlers.PullRequestHandler
	ReviewHandler      *handlers.ReviewHandler
	WebhookHandler     *handlers.WebhookHandler
	ModelsHandler      *handlers.ModelsHandler
}

// NewRouter creates and configures a Gin engine with all routes and middleware.
func NewRouter(deps RouterDeps, log zerolog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Per-IP rate limiting (in-memory; swappable for Redis in multi-instance setups)
	// and webhook delivery de-duplication.
	rateLimiter := middleware.NewRateLimiter(20, 40)
	webhookDeduper := middleware.NewDeduper(10 * time.Minute)

	// Global middleware
	router.Use(middleware.RecoveryMiddleware(log))
	router.Use(middleware.LoggingMiddleware(log))
	router.Use(middleware.MetricsMiddleware())
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Use(middleware.CORSMiddleware())

	// API routes
	api := router.Group("/api")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Prometheus metrics (text exposition format)
		api.GET("/metrics", func(c *gin.Context) {
			c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
			c.String(http.StatusOK, metrics.Render())
		})

		// Available LLM models
		api.GET("/models", deps.ModelsHandler.List)

		// Webhooks (deduplicated by GitHub delivery ID to ignore retries)
		api.POST("/webhooks/github", middleware.WebhookDedupMiddleware(webhookDeduper), deps.WebhookHandler.HandleWebhook)

		// Dashboard
		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/stats", deps.DashboardHandler.GetStats)
			dashboard.GET("/activity", deps.DashboardHandler.GetActivity)
		}

		// Repositories
		repos := api.Group("/repositories")
		{
			repos.GET("", deps.RepositoryHandler.List)
			repos.POST("", deps.RepositoryHandler.Create)
			repos.GET("/:id", deps.RepositoryHandler.GetByID)
			repos.PUT("/:id/settings", deps.RepositoryHandler.UpdateSettings)
			repos.PATCH("/:id/settings", deps.RepositoryHandler.UpdateSettings)
			repos.DELETE("/:id", deps.RepositoryHandler.Delete)
			repos.POST("/:id/sync", deps.RepositoryHandler.Sync)
		}

		// Pull Requests
		prs := api.Group("/pull-requests")
		{
			prs.GET("", deps.PullRequestHandler.List)
			prs.GET("/:id", deps.PullRequestHandler.GetByID)
			prs.GET("/:id/files", deps.PullRequestHandler.GetFiles)
			prs.POST("/:id/trigger-review", deps.PullRequestHandler.TriggerReview)
		}

		// Reviews
		reviews := api.Group("/reviews")
		{
			reviews.GET("", deps.ReviewHandler.List)
			reviews.GET("/:id", deps.ReviewHandler.GetByID)
			reviews.GET("/:id/logs", deps.ReviewHandler.GetLogs)
			reviews.POST("/:id/retry", deps.ReviewHandler.RetryReview)
		}
	}

	return router
}
