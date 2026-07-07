package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	apperrors "github.com/codepilot-ai/codepilot-ai/pkg/errors"
	"github.com/codepilot-ai/codepilot-ai/pkg/logger"
)

// RepositoryService handles all database operations for repositories.
type RepositoryService struct {
	db *sql.DB
}

// NewRepositoryService creates a new RepositoryService.
func NewRepositoryService(db *sql.DB) *RepositoryService {
	return &RepositoryService{db: db}
}

// scanRepository scans a single repository row into a Repository model.
func scanRepository(row interface{ Scan(dest ...interface{}) error }) (*models.Repository, error) {
	var repo models.Repository
	err := row.Scan(
		&repo.ID,
		&repo.GitHubID,
		&repo.Owner,
		&repo.Name,
		&repo.FullName,
		&repo.Description,
		&repo.DefaultBranch,
		&repo.Language,
		&repo.IsActive,
		&repo.WebhookID,
		&repo.SettingsRaw,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	repo.PopulateComputed()
	return &repo, nil
}

const repoColumns = `id, github_id, owner, name, full_name, description, default_branch,
	language, is_active, webhook_id, settings, created_at, updated_at`

// List returns all repositories, ordered by creation date descending.
func (s *RepositoryService) List(ctx context.Context) ([]models.Repository, error) {
	query := fmt.Sprintf(`SELECT %s FROM repositories ORDER BY created_at DESC`, repoColumns)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, apperrors.NewInternal("failed to list repositories", err)
	}
	defer rows.Close()

	var repos []models.Repository
	for rows.Next() {
		repo, err := scanRepository(rows)
		if err != nil {
			return nil, apperrors.NewInternal("failed to scan repository", err)
		}
		repos = append(repos, *repo)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating repository rows", err)
	}

	return repos, nil
}

// GetByID retrieves a single repository by its UUID.
func (s *RepositoryService) GetByID(ctx context.Context, id string) (*models.Repository, error) {
	query := fmt.Sprintf(`SELECT %s FROM repositories WHERE id = $1`, repoColumns)

	row := s.db.QueryRowContext(ctx, query, id)
	repo, err := scanRepository(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("repository", id)
		}
		return nil, apperrors.NewInternal("failed to get repository", err)
	}
	return repo, nil
}

// GetByFullName retrieves a repository by its full_name (e.g., "owner/repo").
func (s *RepositoryService) GetByFullName(ctx context.Context, fullName string) (*models.Repository, error) {
	query := fmt.Sprintf(`SELECT %s FROM repositories WHERE full_name = $1`, repoColumns)

	row := s.db.QueryRowContext(ctx, query, fullName)
	repo, err := scanRepository(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("repository", fullName)
		}
		return nil, apperrors.NewInternal("failed to get repository by full name", err)
	}
	return repo, nil
}

// Create inserts a new repository into the database.
func (s *RepositoryService) Create(ctx context.Context, req *models.CreateRepositoryRequest) (*models.Repository, error) {
	log := logger.WithContext(ctx)
	if req.GitHubID <= 0 || req.Owner == "" || req.Name == "" || req.FullName == "" {
		return nil, apperrors.NewValidation("verified GitHub repository metadata is required")
	}

	defaultBranch := req.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	defaultSettings, _ := json.Marshal(models.DefaultRepositorySettings())

	query := fmt.Sprintf(`
		INSERT INTO repositories (github_id, owner, name, full_name, description, default_branch, language, settings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING %s`, repoColumns)

	row := s.db.QueryRowContext(ctx, query,
		req.GitHubID,
		req.Owner,
		req.Name,
		req.FullName,
		sql.NullString{String: req.Description, Valid: req.Description != ""},
		defaultBranch,
		sql.NullString{String: req.Language, Valid: req.Language != ""},
		defaultSettings,
	)

	repo, err := scanRepository(row)
	if err != nil {
		// Check for unique constraint violation
		if isUniqueViolation(err) {
			return nil, apperrors.NewConflict("repository", fmt.Sprintf("repository '%s' already exists", req.FullName))
		}
		return nil, apperrors.NewInternal("failed to create repository", err)
	}

	log.Info().Str("repository_id", repo.ID).Str("full_name", repo.FullName).Msg("repository created")
	return repo, nil
}

// UpdateSettings updates the JSONB settings for a repository.
func (s *RepositoryService) UpdateSettings(ctx context.Context, id string, req *models.UpdateRepositorySettingsRequest) error {
	log := logger.WithContext(ctx)

	// Fetch current settings
	var currentRaw json.RawMessage
	err := s.db.QueryRowContext(ctx, `SELECT settings FROM repositories WHERE id = $1`, id).Scan(&currentRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("repository", id)
		}
		return apperrors.NewInternal("failed to fetch current settings", err)
	}

	current := models.DefaultRepositorySettings()
	if len(currentRaw) > 0 {
		_ = json.Unmarshal(currentRaw, &current)
	}

	// Merge: only update fields that are non-nil in the request
	if req.LLMModel != nil {
		current.LLMModel = *req.LLMModel
	}
	if req.AutoReview != nil {
		current.AutoReview = *req.AutoReview
	}
	if req.ReviewOnDraft != nil {
		current.ReviewOnDraft = *req.ReviewOnDraft
	}
	if req.ExcludePatterns != nil {
		current.ExcludePatterns = req.ExcludePatterns
	}
	if req.MaxFilesPerReview != nil {
		if *req.MaxFilesPerReview < 1 || *req.MaxFilesPerReview > 200 {
			return apperrors.NewValidationField("max_files_per_review", "must be between 1 and 200")
		}
		current.MaxFilesPerReview = *req.MaxFilesPerReview
	}

	settingsJSON, err := json.Marshal(current)
	if err != nil {
		return apperrors.NewInternal("failed to marshal settings", err)
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE repositories SET settings = $1, updated_at = $2 WHERE id = $3`,
		settingsJSON, time.Now().UTC(), id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update repository settings", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperrors.NewNotFound("repository", id)
	}

	log.Info().Str("repository_id", id).Msg("repository settings updated")
	return nil
}

// Delete removes a repository from the database.
func (s *RepositoryService) Delete(ctx context.Context, id string) error {
	log := logger.WithContext(ctx)

	result, err := s.db.ExecContext(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	if err != nil {
		return apperrors.NewInternal("failed to delete repository", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperrors.NewNotFound("repository", id)
	}

	log.Info().Str("repository_id", id).Msg("repository deleted")
	return nil
}

// SetActive enables or disables a repository for automatic reviews.
func (s *RepositoryService) SetActive(ctx context.Context, id string, active bool) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE repositories SET is_active = $1, updated_at = $2 WHERE id = $3`,
		active, time.Now().UTC(), id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update repository active status", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperrors.NewNotFound("repository", id)
	}

	return nil
}

// UpdateWebhookID stores the GitHub webhook ID returned after auto-registering the hook.
func (s *RepositoryService) UpdateWebhookID(ctx context.Context, id string, webhookID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE repositories SET webhook_id = $1, updated_at = $2 WHERE id = $3`,
		webhookID, time.Now().UTC(), id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update webhook id", err)
	}
	return nil
}

// isUniqueViolation checks if a database error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL unique_violation error code is 23505
	return contains(err.Error(), "duplicate key") || contains(err.Error(), "23505")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
