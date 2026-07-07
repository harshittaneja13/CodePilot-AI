// Package handlers provides HTTP handlers for the CodePilot AI API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/codepilot-ai/codepilot-ai/internal/services"
)

// DashboardHandler handles dashboard-related HTTP requests.
type DashboardHandler struct {
	dashboardService *services.DashboardService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(ds *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: ds}
}

// GetStats returns aggregate dashboard statistics.
func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.dashboardService.GetStats(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetActivity returns recent pipeline milestones across all reviews for the live feed.
func (h *DashboardHandler) GetActivity(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	events, err := h.dashboardService.ListRecentActivity(c.Request.Context(), limit)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, events)
}
