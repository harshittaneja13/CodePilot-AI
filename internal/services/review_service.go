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

// ReviewService handles all database operations for reviews and review comments.
type ReviewService struct {
	db *sql.DB
}

// NewReviewService creates a new ReviewService.
func NewReviewService(db *sql.DB) *ReviewService {
	return &ReviewService{db: db}
}

const reviewColumns = `id, pull_request_id, status, summary, total_comments,
	critical_count, high_count, medium_count, low_count, llm_model,
	tokens_used, input_tokens, output_tokens, cost_usd, processing_time_ms,
	error_message, started_at, completed_at, created_at`

// scanReview scans a single review row into a Review model.
func scanReview(row interface{ Scan(dest ...interface{}) error }) (*models.Review, error) {
	var r models.Review
	err := row.Scan(
		&r.ID,
		&r.PullRequestID,
		&r.Status,
		&r.Summary,
		&r.TotalComments,
		&r.CriticalCount,
		&r.HighCount,
		&r.MediumCount,
		&r.LowCount,
		&r.LLMModel,
		&r.TokensUsed,
		&r.InputTokens,
		&r.OutputTokens,
		&r.CostUSD,
		&r.ProcessingTimeMs,
		&r.ErrorMessage,
		&r.StartedAt,
		&r.CompletedAt,
		&r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.PopulateComputed()
	return &r, nil
}

const commentColumns = `id, review_id, file_path, line_number, severity, title,
	explanation, why_it_matters, suggestion, code_snippet, published, created_at`

// scanComment scans a single review comment row into a ReviewComment model.
func scanComment(row interface{ Scan(dest ...interface{}) error }) (*models.ReviewComment, error) {
	var c models.ReviewComment
	err := row.Scan(
		&c.ID,
		&c.ReviewID,
		&c.FilePath,
		&c.LineNumber,
		&c.Severity,
		&c.Title,
		&c.Explanation,
		&c.WhyItMatters,
		&c.Suggestion,
		&c.CodeSnippet,
		&c.Published,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.PopulateComputed()
	return &c, nil
}

// Create inserts a new review record into the database.
func (s *ReviewService) Create(ctx context.Context, review *models.Review) (*models.Review, error) {
	log := logger.WithContext(ctx)

	query := fmt.Sprintf(`
		INSERT INTO reviews (pull_request_id, status, llm_model, started_at)
		VALUES ($1, $2, $3, $4)
		RETURNING %s`, reviewColumns)

	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, query,
		review.PullRequestID,
		models.ReviewStatusPending,
		sql.NullString{String: review.LLMModelStr, Valid: review.LLMModelStr != ""},
		sql.NullTime{Time: now, Valid: true},
	)

	result, err := scanReview(row)
	if err != nil {
		return nil, apperrors.NewInternal("failed to create review", err)
	}

	log.Info().Str("review_id", result.ID).Str("pr_id", result.PullRequestID).Msg("review created")
	return result, nil
}

// GetByID retrieves a review by ID along with its comments.
func (s *ReviewService) GetByID(ctx context.Context, id string) (*models.ReviewWithComments, error) {
	// Fetch the review
	query := fmt.Sprintf(`SELECT %s FROM reviews WHERE id = $1`, reviewColumns)
	row := s.db.QueryRowContext(ctx, query, id)
	review, err := scanReview(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("review", id)
		}
		return nil, apperrors.NewInternal("failed to get review", err)
	}

	// Fetch associated comments
	commentsQuery := fmt.Sprintf(`SELECT %s FROM review_comments WHERE review_id = $1 ORDER BY created_at ASC`, commentColumns)
	rows, err := s.db.QueryContext(ctx, commentsQuery, id)
	if err != nil {
		return nil, apperrors.NewInternal("failed to list review comments", err)
	}
	defer rows.Close()

	var comments []models.ReviewComment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, apperrors.NewInternal("failed to scan review comment", err)
		}
		comments = append(comments, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating review comment rows", err)
	}

	if comments == nil {
		comments = []models.ReviewComment{}
	}

	return &models.ReviewWithComments{
		Review:   *review,
		Comments: comments,
	}, nil
}

// ListByPR returns all reviews for a given pull request, ordered by creation date descending.
func (s *ReviewService) ListByPR(ctx context.Context, prID string) ([]models.Review, error) {
	query := fmt.Sprintf(`SELECT %s FROM reviews WHERE pull_request_id = $1 ORDER BY created_at DESC`, reviewColumns)

	rows, err := s.db.QueryContext(ctx, query, prID)
	if err != nil {
		return nil, apperrors.NewInternal("failed to list reviews by PR", err)
	}
	defer rows.Close()

	var reviews []models.Review
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, apperrors.NewInternal("failed to scan review", err)
		}
		reviews = append(reviews, *r)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating review rows", err)
	}

	return reviews, nil
}

// ListRecent returns the most recent reviews across all pull requests.
func (s *ReviewService) ListRecent(ctx context.Context, limit int) ([]models.Review, error) {
	query := fmt.Sprintf(`SELECT %s FROM reviews ORDER BY created_at DESC LIMIT $1`, reviewColumns)

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, apperrors.NewInternal("failed to list recent reviews", err)
	}
	defer rows.Close()

	var reviews []models.Review
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, apperrors.NewInternal("failed to scan review", err)
		}
		reviews = append(reviews, *r)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating review rows", err)
	}

	return reviews, nil
}

// UpdateStatus updates the status, summary, and error message of a review.
func (s *ReviewService) UpdateStatus(ctx context.Context, id, status, summary, errMsg string) error {
	log := logger.WithContext(ctx)

	var completedAt sql.NullTime
	if status == models.ReviewStatusCompleted || status == models.ReviewStatusFailed {
		completedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE reviews SET status = $1, summary = $2, error_message = $3, completed_at = $4
		 WHERE id = $5`,
		status,
		sql.NullString{String: summary, Valid: summary != ""},
		sql.NullString{String: errMsg, Valid: errMsg != ""},
		completedAt,
		id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update review status", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperrors.NewNotFound("review", id)
	}

	log.Info().Str("review_id", id).Str("status", status).Msg("review status updated")
	return nil
}

// AddComment inserts a new review comment.
func (s *ReviewService) AddComment(ctx context.Context, comment *models.ReviewComment) (*models.ReviewComment, error) {
	query := fmt.Sprintf(`
		INSERT INTO review_comments (review_id, file_path, line_number, severity, title,
			explanation, why_it_matters, suggestion, code_snippet, published)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING %s`, commentColumns)

	row := s.db.QueryRowContext(ctx, query,
		comment.ReviewID,
		comment.FilePath,
		comment.LineNumber,
		comment.Severity,
		comment.Title,
		comment.Explanation,
		sql.NullString{String: comment.WhyItMattersStr, Valid: comment.WhyItMattersStr != ""},
		sql.NullString{String: comment.SuggestionStr, Valid: comment.SuggestionStr != ""},
		sql.NullString{String: comment.CodeSnippetStr, Valid: comment.CodeSnippetStr != ""},
		comment.Published,
	)

	result, err := scanComment(row)
	if err != nil {
		return nil, apperrors.NewInternal("failed to add review comment", err)
	}

	return result, nil
}

// AddExecutionLog inserts a new execution log entry.
func (s *ReviewService) AddExecutionLog(ctx context.Context, execLog *models.ExecutionLog) error {
	metadataJSON := json.RawMessage("{}")
	if len(execLog.Metadata) > 0 {
		metadataJSON = execLog.Metadata
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO execution_logs (review_id, step, status, message, metadata, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		execLog.ReviewID,
		execLog.Step,
		execLog.Status,
		sql.NullString{String: execLog.MessageStr, Valid: execLog.MessageStr != ""},
		metadataJSON,
		execLog.DurationMs,
	)
	if err != nil {
		return apperrors.NewInternal("failed to add execution log", err)
	}

	return nil
}

// UpdateCounts updates the comment counts and processing time for a review.
func (s *ReviewService) UpdateCounts(ctx context.Context, id string, total, critical, high, medium, low, tokensUsed int, processingTimeMs int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reviews SET total_comments = $1, critical_count = $2, high_count = $3,
		 medium_count = $4, low_count = $5, tokens_used = $6, processing_time_ms = $7
		 WHERE id = $8`,
		total, critical, high, medium, low, tokensUsed, processingTimeMs, id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update review counts", err)
	}
	return nil
}

// UpdateUsage persists the input/output token split and computed USD cost for a review.
func (s *ReviewService) UpdateUsage(ctx context.Context, id string, inputTokens, outputTokens int, costUSD float64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reviews SET input_tokens = $1, output_tokens = $2, cost_usd = $3 WHERE id = $4`,
		inputTokens, outputTokens, costUSD, id,
	)
	if err != nil {
		return apperrors.NewInternal("failed to update review usage", err)
	}
	return nil
}

// ListExecutionLogs returns the ordered pipeline steps (the agent trace) for a review.
func (s *ReviewService) ListExecutionLogs(ctx context.Context, reviewID string) ([]models.ExecutionLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, review_id, step, status, message, metadata, duration_ms, created_at
		 FROM execution_logs WHERE review_id = $1 ORDER BY created_at ASC`, reviewID)
	if err != nil {
		return nil, apperrors.NewInternal("failed to list execution logs", err)
	}
	defer rows.Close()

	var logs []models.ExecutionLog
	for rows.Next() {
		var el models.ExecutionLog
		var meta []byte
		if err := rows.Scan(&el.ID, &el.ReviewID, &el.Step, &el.Status, &el.Message, &meta, &el.DurationMs, &el.CreatedAt); err != nil {
			return nil, apperrors.NewInternal("failed to scan execution log", err)
		}
		el.Metadata = meta
		el.PopulateComputed()
		logs = append(logs, el)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating execution logs", err)
	}
	return logs, nil
}
