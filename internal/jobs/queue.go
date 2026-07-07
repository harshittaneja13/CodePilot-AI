// Package jobs implements the durable PostgreSQL queue used for PR reviews.
package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Processor func(context.Context, string, string, int, string) error

type Job struct {
	ID        string
	Owner     string
	Repo      string
	PRNumber  int
	Action    string
	Attempts  int
	MaxAttempts int
}

type Queue struct {
	db        *sql.DB
	processor Processor
	workers   int
	logger    zerolog.Logger
	wg        sync.WaitGroup
}

func NewQueue(db *sql.DB, processor Processor, workers int, logger zerolog.Logger) *Queue {
	if workers < 1 {
		workers = 2
	}
	return &Queue{db: db, processor: processor, workers: workers, logger: logger.With().Str("component", "review-queue").Logger()}
}

// Enqueue is idempotent for a GitHub delivery ID.
func (q *Queue) Enqueue(ctx context.Context, deliveryID, owner, repo string, prNumber int, action string) error {
	if deliveryID == "" {
		return fmt.Errorf("delivery ID is required")
	}
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO review_jobs (delivery_id, owner, repository, pull_request_number, action)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (delivery_id) DO NOTHING`, deliveryID, owner, repo, prNumber, action)
	if err != nil {
		return fmt.Errorf("enqueue review job: %w", err)
	}
	return nil
}

func (q *Queue) EnqueueManual(ctx context.Context, owner, repo string, prNumber int) error {
	return q.Enqueue(ctx, "manual-"+uuid.NewString(), owner, repo, prNumber, "manual_retry")
}

func (q *Queue) Start(ctx context.Context) error {
	if q.processor == nil {
		return fmt.Errorf("review job processor is required")
	}
	// A process may have stopped after claiming work. Make stale jobs visible.
	if _, err := q.db.ExecContext(ctx, `
		UPDATE review_jobs SET status = 'pending', locked_at = NULL, available_at = NOW(), updated_at = NOW()
		WHERE status = 'processing' AND locked_at < NOW() - INTERVAL '15 minutes'`); err != nil {
		return fmt.Errorf("recover stale review jobs: %w", err)
	}
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx, i+1)
	}
	return nil
}

func (q *Queue) Wait() { q.wg.Wait() }

func (q *Queue) worker(ctx context.Context, workerID int) {
	defer q.wg.Done()
	log := q.logger.With().Int("worker", workerID).Logger()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		job, err := q.claim(ctx)
		if err != nil && err != sql.ErrNoRows && ctx.Err() == nil {
			log.Error().Err(err).Msg("failed to claim review job")
		}
		if job != nil {
			q.process(ctx, log, job)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (q *Queue) claim(ctx context.Context) (*Job, error) {
	row := q.db.QueryRowContext(ctx, `
		WITH next_job AS (
			SELECT id FROM review_jobs
			WHERE status = 'pending' AND available_at <= NOW()
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE review_jobs j
		SET status = 'processing', attempts = attempts + 1, locked_at = NOW(), updated_at = NOW()
		FROM next_job WHERE j.id = next_job.id
		RETURNING j.id, j.owner, j.repository, j.pull_request_number, j.action, j.attempts, j.max_attempts`)
	var job Job
	if err := row.Scan(&job.ID, &job.Owner, &job.Repo, &job.PRNumber, &job.Action, &job.Attempts, &job.MaxAttempts); err != nil {
		return nil, err
	}
	return &job, nil
}

func (q *Queue) process(parent context.Context, log zerolog.Logger, job *Job) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	err := q.processor(ctx, job.Owner, job.Repo, job.PRNumber, job.Action)
	if err == nil {
		_, updateErr := q.db.ExecContext(context.Background(), `
			UPDATE review_jobs SET status = 'completed', locked_at = NULL, last_error = NULL, updated_at = NOW()
			WHERE id = $1`, job.ID)
		if updateErr != nil {
			log.Error().Err(updateErr).Str("job_id", job.ID).Msg("failed to complete review job")
		}
		return
	}

	status := "pending"
	if job.Attempts >= job.MaxAttempts {
		status = "failed"
	}
	backoff := time.Duration(1<<min(job.Attempts, 6)) * time.Minute
	_, updateErr := q.db.ExecContext(context.Background(), `
		UPDATE review_jobs SET status = $1, available_at = $2, locked_at = NULL, last_error = $3, updated_at = NOW()
		WHERE id = $4`, status, time.Now().UTC().Add(backoff), err.Error(), job.ID)
	if updateErr != nil {
		log.Error().Err(updateErr).Str("job_id", job.ID).Msg("failed to reschedule review job")
	}
	log.Error().Err(err).Str("job_id", job.ID).Int("attempt", job.Attempts).Str("next_status", status).Msg("review job failed")
}
