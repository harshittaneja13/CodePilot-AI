// Package services provides data-access services for the CodePilot AI application.
package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	apperrors "github.com/codepilot-ai/codepilot-ai/pkg/errors"
	"github.com/codepilot-ai/codepilot-ai/pkg/logger"
)

// PullRequestService handles all database operations for pull requests.
type PullRequestService struct {
	db *sql.DB
}

// NewPullRequestService creates a new PullRequestService.
func NewPullRequestService(db *sql.DB) *PullRequestService {
	return &PullRequestService{db: db}
}

const prColumns = `id, repository_id, github_number, title, body, state, author,
	head_branch, base_branch, head_sha, additions, deletions, changed_files,
	github_url, created_at, updated_at`

// scanPullRequest scans a single pull request row into a PullRequest model.
func scanPullRequest(row interface{ Scan(dest ...interface{}) error }) (*models.PullRequest, error) {
	var pr models.PullRequest
	err := row.Scan(
		&pr.ID,
		&pr.RepositoryID,
		&pr.GitHubNumber,
		&pr.Title,
		&pr.Body,
		&pr.State,
		&pr.Author,
		&pr.HeadBranch,
		&pr.BaseBranch,
		&pr.HeadSHA,
		&pr.Additions,
		&pr.Deletions,
		&pr.ChangedFiles,
		&pr.GitHubURL,
		&pr.CreatedAt,
		&pr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	pr.PopulateComputed()
	return &pr, nil
}

// List returns pull requests with optional filtering by repository, ordered by creation date descending.
func (s *PullRequestService) List(ctx context.Context, repoID *string, limit, offset int) ([]models.PullRequest, error) {
	var query string
	var args []interface{}

	if repoID != nil && *repoID != "" {
		query = fmt.Sprintf(`SELECT %s FROM pull_requests WHERE repository_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, prColumns)
		args = []interface{}{*repoID, limit, offset}
	} else {
		query = fmt.Sprintf(`SELECT %s FROM pull_requests ORDER BY created_at DESC LIMIT $1 OFFSET $2`, prColumns)
		args = []interface{}{limit, offset}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternal("failed to list pull requests", err)
	}
	defer rows.Close()

	var prs []models.PullRequest
	for rows.Next() {
		pr, err := scanPullRequest(rows)
		if err != nil {
			return nil, apperrors.NewInternal("failed to scan pull request", err)
		}
		prs = append(prs, *pr)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating pull request rows", err)
	}

	return prs, nil
}

// GetByID retrieves a single pull request by its UUID.
func (s *PullRequestService) GetByID(ctx context.Context, id string) (*models.PullRequest, error) {
	query := fmt.Sprintf(`SELECT %s FROM pull_requests WHERE id = $1`, prColumns)

	row := s.db.QueryRowContext(ctx, query, id)
	pr, err := scanPullRequest(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("pull_request", id)
		}
		return nil, apperrors.NewInternal("failed to get pull request", err)
	}
	return pr, nil
}

// GetByRepoAndNumber retrieves a pull request by repository ID and GitHub number.
func (s *PullRequestService) GetByRepoAndNumber(ctx context.Context, repoID string, number int) (*models.PullRequest, error) {
	query := fmt.Sprintf(`SELECT %s FROM pull_requests WHERE repository_id = $1 AND github_number = $2`, prColumns)

	row := s.db.QueryRowContext(ctx, query, repoID, number)
	pr, err := scanPullRequest(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("pull_request", fmt.Sprintf("repo=%s number=%d", repoID, number))
		}
		return nil, apperrors.NewInternal("failed to get pull request by repo and number", err)
	}
	return pr, nil
}

// Upsert inserts a new pull request or updates an existing one on conflict of (repository_id, github_number).
func (s *PullRequestService) Upsert(ctx context.Context, pr *models.PullRequest) (*models.PullRequest, error) {
	log := logger.WithContext(ctx)

	now := time.Now().UTC()
	query := fmt.Sprintf(`
		INSERT INTO pull_requests (
			repository_id, github_number, title, body, state, author,
			head_branch, base_branch, head_sha, additions, deletions,
			changed_files, github_url, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (repository_id, github_number) DO UPDATE SET
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			state = EXCLUDED.state,
			head_branch = EXCLUDED.head_branch,
			base_branch = EXCLUDED.base_branch,
			head_sha = EXCLUDED.head_sha,
			additions = EXCLUDED.additions,
			deletions = EXCLUDED.deletions,
			changed_files = EXCLUDED.changed_files,
			github_url = EXCLUDED.github_url,
			updated_at = EXCLUDED.updated_at
		RETURNING %s`, prColumns)

	row := s.db.QueryRowContext(ctx, query,
		pr.RepositoryID,
		pr.GitHubNumber,
		pr.Title,
		sql.NullString{String: pr.BodyStr, Valid: pr.BodyStr != ""},
		pr.State,
		pr.Author,
		sql.NullString{String: pr.HeadBranchStr, Valid: pr.HeadBranchStr != ""},
		sql.NullString{String: pr.BaseBranchStr, Valid: pr.BaseBranchStr != ""},
		sql.NullString{String: pr.HeadSHAStr, Valid: pr.HeadSHAStr != ""},
		pr.Additions,
		pr.Deletions,
		pr.ChangedFiles,
		sql.NullString{String: pr.GitHubURLStr, Valid: pr.GitHubURLStr != ""},
		now,
		now,
	)

	result, err := scanPullRequest(row)
	if err != nil {
		return nil, apperrors.NewInternal("failed to upsert pull request", err)
	}

	log.Info().
		Str("pull_request_id", result.ID).
		Int("github_number", result.GitHubNumber).
		Msg("pull request upserted")
	return result, nil
}

// UpdateState updates the state of a pull request.
// UpdateStats persists diff statistics fetched from the GitHub individual PR endpoint.
func (s *PullRequestService) UpdateStats(ctx context.Context, id string, additions, deletions, changedFiles int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE pull_requests SET additions = $1, deletions = $2, changed_files = $3, updated_at = $4 WHERE id = $5`,
		additions, deletions, changedFiles, time.Now().UTC(), id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update pull request stats", err)
	}
	return nil
}

func (s *PullRequestService) UpdateState(ctx context.Context, id string, state string) error {
	log := logger.WithContext(ctx)

	result, err := s.db.ExecContext(ctx,
		`UPDATE pull_requests SET state = $1, updated_at = $2 WHERE id = $3`,
		state, time.Now().UTC(), id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update pull request state", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperrors.NewNotFound("pull_request", id)
	}

	log.Info().Str("pull_request_id", id).Str("state", state).Msg("pull request state updated")
	return nil
}
