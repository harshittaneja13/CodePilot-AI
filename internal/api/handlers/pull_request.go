package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/services"
	apperrors "github.com/codepilot-ai/codepilot-ai/pkg/errors"
)

// PullRequestHandler handles pull request HTTP requests.
type PullRequestHandler struct {
	prService     *services.PullRequestService
	reviewService *services.ReviewService
	enrichStats   func(ctx context.Context, prID string) error
	fetchFiles    func(ctx context.Context, prID string) ([]map[string]interface{}, error)
	triggerReview func(ctx context.Context, prID, model string) error
}

// NewPullRequestHandler creates a new PullRequestHandler.
func NewPullRequestHandler(
	ps *services.PullRequestService,
	rs *services.ReviewService,
	enrichStats func(ctx context.Context, prID string) error,
	fetchFiles func(ctx context.Context, prID string) ([]map[string]interface{}, error),
	triggerReview func(ctx context.Context, prID, model string) error,
) *PullRequestHandler {
	return &PullRequestHandler{
		prService:     ps,
		reviewService: rs,
		enrichStats:   enrichStats,
		fetchFiles:    fetchFiles,
		triggerReview: triggerReview,
	}
}

// GetFiles returns the list of files changed in a pull request, fetched live from GitHub.
func (h *PullRequestHandler) GetFiles(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("pull request id is required"))
		return
	}
	if h.fetchFiles == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "file listing unavailable"})
		return
	}
	files, err := h.fetchFiles(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	if files == nil {
		files = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, files)
}

// TriggerReview enqueues an AI review job for the given pull request.
// An optional JSON body `{"model":"<name>"}` overrides the repo-level model for this run.
func (h *PullRequestHandler) TriggerReview(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("pull request id is required"))
		return
	}
	if h.triggerReview == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "review engine unavailable"})
		return
	}
	var body struct {
		Model string `json:"model"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.triggerReview(c.Request.Context(), id, body.Model); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "review queued"})
}

// List returns pull requests, optionally filtered by repo_id query param.
func (h *PullRequestHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var repoID *string
	if rid := c.Query("repo_id"); rid != "" {
		repoID = &rid
	}

	prs, err := h.prService.List(c.Request.Context(), repoID, limit, offset)
	if err != nil {
		handleError(c, err)
		return
	}

	if prs == nil {
		prs = []models.PullRequest{}
	}

	c.JSON(http.StatusOK, prs)
}

// GetByID returns a single pull request by UUID, including its reviews.
func (h *PullRequestHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("pull request id is required"))
		return
	}

	pr, err := h.prService.GetByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	// Lazily fetch diff stats from GitHub if they were missing at sync time.
	if pr.Additions == 0 && pr.Deletions == 0 && pr.ChangedFiles == 0 && h.enrichStats != nil {
		if enrichErr := h.enrichStats(c.Request.Context(), id); enrichErr == nil {
			// Re-read the updated record so the response includes real stats.
			if updated, refetchErr := h.prService.GetByID(c.Request.Context(), id); refetchErr == nil {
				pr = updated
			}
		}
	}

	reviews, err := h.reviewService.ListByPR(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	if reviews == nil {
		reviews = []models.Review{}
	}

	c.JSON(http.StatusOK, gin.H{
		"pull_request": pr,
		"reviews":      reviews,
	})
}
