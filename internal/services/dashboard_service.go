package services

import (
	"context"
	"database/sql"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	apperrors "github.com/codepilot-ai/codepilot-ai/pkg/errors"
)

// DashboardService provides aggregate data for the dashboard.
type DashboardService struct {
	db *sql.DB
}

// NewDashboardService creates a new DashboardService.
func NewDashboardService(db *sql.DB) *DashboardService {
	return &DashboardService{db: db}
}

// GetStats returns aggregate statistics for the dashboard.
func (s *DashboardService) GetStats(ctx context.Context) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{
		ReviewActivity: []models.ReviewActivity{},
		RecentReviews:  []models.DashboardReview{},
	}

	// Scalar counts
	queries := []struct {
		dest  interface{}
		query string
	}{
		{&stats.TotalRepositories, `SELECT COUNT(*) FROM repositories`},
		{&stats.TotalPullRequests, `SELECT COUNT(*) FROM pull_requests`},
		{&stats.TotalReviews, `SELECT COUNT(*) FROM reviews`},
		{&stats.TotalFindings, `SELECT COUNT(*) FROM review_comments`},
		{&stats.CriticalFindings, `SELECT COUNT(*) FROM review_comments WHERE severity = 'critical'`},
		{&stats.HighFindings, `SELECT COUNT(*) FROM review_comments WHERE severity = 'high'`},
		{&stats.MediumFindings, `SELECT COUNT(*) FROM review_comments WHERE severity = 'medium'`},
		{&stats.LowFindings, `SELECT COUNT(*) FROM review_comments WHERE severity = 'low'`},
		{&stats.ReviewsToday, `SELECT COUNT(*) FROM reviews WHERE created_at >= CURRENT_DATE`},
		{&stats.ReviewsThisWeek, `SELECT COUNT(*) FROM reviews WHERE created_at >= date_trunc('week', NOW())`},
		{&stats.TotalCostUSD, `SELECT COALESCE(SUM(cost_usd), 0)::float8 FROM reviews`},
	}

	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.query).Scan(q.dest); err != nil {
			return nil, apperrors.NewInternal("failed to query dashboard stat", err)
		}
	}

	// Average processing time for completed reviews
	var avgTime sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT AVG(processing_time_ms)::float8 FROM reviews WHERE status = 'completed' AND processing_time_ms > 0`,
	).Scan(&avgTime)
	if err != nil {
		return nil, apperrors.NewInternal("failed to query average review time", err)
	}
	if avgTime.Valid {
		stats.AvgReviewTimeMs = int64(avgTime.Float64)
	}

	// Review activity for the past 14 days
	activityRows, err := s.db.QueryContext(ctx, `
		SELECT
			to_char(d::date, 'YYYY-MM-DD') AS date,
			COALESCE(r.review_count, 0)   AS reviews,
			COALESCE(r.finding_count, 0)  AS findings
		FROM generate_series(
			CURRENT_DATE - INTERVAL '13 days',
			CURRENT_DATE,
			INTERVAL '1 day'
		) AS d
		LEFT JOIN (
			SELECT
				DATE_TRUNC('day', rv.created_at) AS day,
				COUNT(DISTINCT rv.id)             AS review_count,
				COUNT(rc.id)                      AS finding_count
			FROM reviews rv
			LEFT JOIN review_comments rc ON rc.review_id = rv.id
			GROUP BY day
		) r ON r.day = d::date
		ORDER BY d`)
	if err != nil {
		return nil, apperrors.NewInternal("failed to query review activity", err)
	}
	defer activityRows.Close()

	for activityRows.Next() {
		var activity models.ReviewActivity
		if err := activityRows.Scan(&activity.Date, &activity.Reviews, &activity.Findings); err != nil {
			return nil, apperrors.NewInternal("failed to scan review activity row", err)
		}
		stats.ReviewActivity = append(stats.ReviewActivity, activity)
	}
	if err := activityRows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating review activity rows", err)
	}

	// Recent reviews with repository and PR context (JOIN)
	recentRows, err := s.db.QueryContext(ctx, `
		SELECT
			rv.id,
			rv.pull_request_id,
			pr.repository_id,
			repo.full_name,
			pr.github_number,
			pr.title,
			rv.status,
			COALESCE(rv.llm_model, ''),
			rv.total_comments,
			rv.critical_count,
			rv.high_count,
			rv.medium_count,
			rv.low_count,
			rv.tokens_used,
			COALESCE(rv.cost_usd, 0),
			COALESCE(rv.processing_time_ms, 0),
			COALESCE(rv.error_message, ''),
			rv.started_at,
			rv.completed_at,
			rv.created_at
		FROM reviews rv
		JOIN pull_requests pr ON pr.id = rv.pull_request_id
		JOIN repositories repo ON repo.id = pr.repository_id
		ORDER BY rv.created_at DESC
		LIMIT 10`)
	if err != nil {
		return nil, apperrors.NewInternal("failed to fetch recent reviews", err)
	}
	defer recentRows.Close()

	for recentRows.Next() {
		var dr models.DashboardReview
		var startedAt, completedAt sql.NullTime
		err := recentRows.Scan(
			&dr.ID,
			&dr.PullRequestID,
			&dr.RepositoryID,
			&dr.RepositoryName,
			&dr.PRNumber,
			&dr.PRTitle,
			&dr.Status,
			&dr.LLMModel,
			&dr.TotalFindings,
			&dr.CriticalCount,
			&dr.HighCount,
			&dr.MediumCount,
			&dr.LowCount,
			&dr.TokensUsed,
			&dr.CostUSD,
			&dr.ProcessingTimeMs,
			&dr.ErrorMessage,
			&startedAt,
			&completedAt,
			&dr.CreatedAt,
		)
		if err != nil {
			return nil, apperrors.NewInternal("failed to scan recent review row", err)
		}
		if startedAt.Valid {
			dr.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			dr.CompletedAt = &completedAt.Time
		}
		stats.RecentReviews = append(stats.RecentReviews, dr)
	}
	if err := recentRows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating recent review rows", err)
	}

	return stats, nil
}

// ListRecentActivity returns recent pipeline milestones across all reviews, enriched
// with PR/repository context. The chatty per-tool-call step is excluded so the feed
// stays readable; the full tool trace remains available per-review.
func (s *DashboardService) ListRecentActivity(ctx context.Context, limit int) ([]models.ActivityEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			el.id,
			el.review_id,
			rv.pull_request_id,
			repo.full_name,
			pr.github_number,
			pr.title,
			el.step,
			el.status,
			COALESCE(el.message, ''),
			COALESCE(el.duration_ms, 0),
			el.created_at
		FROM execution_logs el
		JOIN reviews rv        ON rv.id = el.review_id
		JOIN pull_requests pr  ON pr.id = rv.pull_request_id
		JOIN repositories repo ON repo.id = pr.repository_id
		WHERE el.step <> 'agent_tool'
		ORDER BY el.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, apperrors.NewInternal("failed to fetch activity feed", err)
	}
	defer rows.Close()

	events := make([]models.ActivityEvent, 0, limit)
	for rows.Next() {
		var e models.ActivityEvent
		if err := rows.Scan(
			&e.ID, &e.ReviewID, &e.PullRequestID, &e.RepositoryName,
			&e.PRNumber, &e.PRTitle, &e.Step, &e.Status, &e.Message,
			&e.DurationMs, &e.CreatedAt,
		); err != nil {
			return nil, apperrors.NewInternal("failed to scan activity row", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternal("error iterating activity rows", err)
	}
	return events, nil
}
