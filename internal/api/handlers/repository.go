package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/services"
	apperrors "github.com/codepilot-ai/codepilot-ai/pkg/errors"
)

// RepositoryHandler handles repository-related HTTP requests.
type RepositoryHandler struct {
	repoService *services.RepositoryService
	connectFunc func(context.Context, string, string) (*models.Repository, error)
	syncFunc    func(context.Context, string) (int, error)
}

// NewRepositoryHandler creates a new RepositoryHandler.
func NewRepositoryHandler(
	rs *services.RepositoryService,
	connectFunc func(context.Context, string, string) (*models.Repository, error),
	syncFunc func(context.Context, string) (int, error),
) *RepositoryHandler {
	return &RepositoryHandler{repoService: rs, connectFunc: connectFunc, syncFunc: syncFunc}
}

// List returns all repositories.
func (h *RepositoryHandler) List(c *gin.Context) {
	repos, err := h.repoService.List(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	if repos == nil {
		repos = []models.Repository{}
	}

	c.JSON(http.StatusOK, repos)
}

// GetByID returns a single repository by UUID.
func (h *RepositoryHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("repository id is required"))
		return
	}

	repo, err := h.repoService.GetByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, repo)
}

// Create creates a new repository.
func (h *RepositoryHandler) Create(c *gin.Context) {
	var req models.CreateRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, apperrors.NewValidation(err.Error()))
		return
	}

	var repo *models.Repository
	var err error
	if h.connectFunc != nil {
		repo, err = h.connectFunc(c.Request.Context(), req.Owner, req.Name)
	} else {
		repo, err = h.repoService.Create(c.Request.Context(), &req)
	}
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// UpdateSettings updates a repository's settings.
func (h *RepositoryHandler) UpdateSettings(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("repository id is required"))
		return
	}

	var req models.UpdateRepositorySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, apperrors.NewValidation(err.Error()))
		return
	}

	if err := h.repoService.UpdateSettings(c.Request.Context(), id, &req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

// Sync fetches the latest pull requests from GitHub and upserts them into the DB.
func (h *RepositoryHandler) Sync(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("repository id is required"))
		return
	}
	if h.syncFunc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "sync unavailable: MCP not connected"})
		return
	}
	count, err := h.syncFunc(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"synced": count})
}

// Delete removes a repository.
func (h *RepositoryHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleError(c, apperrors.NewValidation("repository id is required"))
		return
	}

	if err := h.repoService.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "repository deleted"})
}

// handleError maps application errors to HTTP responses using the apperrors package.
func handleError(c *gin.Context, err error) {
	status := apperrors.ToHTTPStatus(err)
	c.JSON(status, apperrors.ToErrorResponse(err))
}
