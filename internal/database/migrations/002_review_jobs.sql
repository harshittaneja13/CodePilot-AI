-- Durable webhook work queue. Jobs survive process restarts and are claimed
-- with SKIP LOCKED so multiple backend replicas can process safely.
CREATE TABLE IF NOT EXISTS review_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    delivery_id VARCHAR(255) UNIQUE NOT NULL,
    owner VARCHAR(255) NOT NULL,
    repository VARCHAR(255) NOT NULL,
    pull_request_number INTEGER NOT NULL CHECK (pull_request_number > 0),
    action VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 4,
    available_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_review_jobs_claim
    ON review_jobs(status, available_at, created_at);
