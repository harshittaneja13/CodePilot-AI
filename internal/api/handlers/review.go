package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/services"
	apperrors "github.com/codepilot-ai/codepilot-ai/pkg/errors"
)

// RetryFunc is a function signature for retrying a review.
type RetryFunc func(ctx interface{}, reviewID string) error

// ReviewHandler handles review-related HTTP requests.
type ReviewHandler struct {
	reviewService *services.ReviewService
	retryFunc     func(reviewID string) error
}

// NewReviewHandler creates a new ReviewHandler.
// retryFunc is called when a user requests a review retry. It may be nil if retry is not supported.
func NewReviewHandler(rs *services.ReviewService, retryFunc func(reviewID string) error) *ReviewHandler {
	return &ReviewHandler{
		reviewService: rs,
		retryFunc:     retryFunc,
	}
}

// List returns recent reviews.
func (h *ReviewHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	reviews, err := h.reviewService.ListRecent(c.Request.Context(), limit)
	if err != nil {
		handleError(c, err)
		return
	}

	if reviews == nil {
		reviews = []models.Review{}
	}

	c.JSON(http.StatusOK, reviews)
}

// GetByID returns a single review by UUID with its comments.
func (h *ReviewHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("review id is required"))
		return
	}

	reviewWithComments, err := h.reviewService.GetByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, reviewWithComments)
}

// GetLogs returns the ordered execution log (agent trace) for a review.
func (h *ReviewHandler) GetLogs(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("review id is required"))
		return
	}

	logs, err := h.reviewService.ListExecutionLogs(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	if logs == nil {
		logs = []models.ExecutionLog{}
	}

	c.JSON(http.StatusOK, logs)
}

// RetryReview triggers a retry for a failed review.
func (h *ReviewHandler) RetryReview(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("review id is required"))
		return
	}

	if h.retryFunc == nil {
		handleError(c, apperrors.NewInternal("retry not configured", nil))
		return
	}

	if err := h.retryFunc(id); err != nil {
		handleError(c, apperrors.NewInternal("failed to retry review", err))
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "review retry initiated"})
}
