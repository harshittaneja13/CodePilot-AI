-- ============================================================================
-- CodePilot AI — Review Jobs Queue Table
-- ============================================================================

CREATE TABLE IF NOT EXISTS review_jobs (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id          TEXT        NOT NULL UNIQUE,
    owner                TEXT        NOT NULL,
    repository           TEXT        NOT NULL,
    pull_request_number  INT         NOT NULL,
    action               TEXT        NOT NULL DEFAULT 'opened',
    status               TEXT        NOT NULL DEFAULT 'pending'
                                     CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts             INT         NOT NULL DEFAULT 0,
    max_attempts         INT         NOT NULL DEFAULT 3,
    locked_at            TIMESTAMPTZ,
    available_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error           TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_review_jobs_status_available ON review_jobs (status, available_at)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_review_jobs_delivery_id ON review_jobs (delivery_id);
