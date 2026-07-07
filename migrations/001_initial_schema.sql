-- ============================================================================
-- CodePilot AI — Initial Database Schema
-- ============================================================================

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- Enums
-- ---------------------------------------------------------------------------

DO $$ BEGIN
    CREATE TYPE review_status AS ENUM ('pending', 'in_progress', 'completed', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE severity_level AS ENUM ('critical', 'high', 'medium', 'low', 'info');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------------------------------------------------------------------------
-- repositories
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS repositories (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id      BIGINT      NOT NULL UNIQUE,
    owner          VARCHAR(255) NOT NULL,
    name           VARCHAR(255) NOT NULL,
    full_name      VARCHAR(511) NOT NULL UNIQUE,
    description    TEXT,
    default_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    language       VARCHAR(100),
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
    webhook_id     BIGINT,
    settings       JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_repositories_owner ON repositories (owner);
CREATE INDEX IF NOT EXISTS idx_repositories_full_name ON repositories (full_name);
CREATE INDEX IF NOT EXISTS idx_repositories_is_active ON repositories (is_active);

-- ---------------------------------------------------------------------------
-- pull_requests
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS pull_requests (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id  UUID        NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    github_number  INT         NOT NULL,
    title          TEXT        NOT NULL,
    body           TEXT,
    state          VARCHAR(50) NOT NULL DEFAULT 'open',
    author         VARCHAR(255) NOT NULL,
    head_branch    VARCHAR(255),
    base_branch    VARCHAR(255),
    head_sha       VARCHAR(40),
    additions      INT         NOT NULL DEFAULT 0,
    deletions      INT         NOT NULL DEFAULT 0,
    changed_files  INT         NOT NULL DEFAULT 0,
    github_url     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (repository_id, github_number)
);

CREATE INDEX IF NOT EXISTS idx_pull_requests_repository_id ON pull_requests (repository_id);
CREATE INDEX IF NOT EXISTS idx_pull_requests_state ON pull_requests (state);
CREATE INDEX IF NOT EXISTS idx_pull_requests_author ON pull_requests (author);
CREATE INDEX IF NOT EXISTS idx_pull_requests_created_at ON pull_requests (created_at DESC);

-- ---------------------------------------------------------------------------
-- reviews
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS reviews (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    pull_request_id     UUID          NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    status              review_status NOT NULL DEFAULT 'pending',
    summary             TEXT,
    total_comments      INT           NOT NULL DEFAULT 0,
    critical_count      INT           NOT NULL DEFAULT 0,
    high_count          INT           NOT NULL DEFAULT 0,
    medium_count        INT           NOT NULL DEFAULT 0,
    low_count           INT           NOT NULL DEFAULT 0,
    llm_model           VARCHAR(100),
    tokens_used         INT           NOT NULL DEFAULT 0,
    processing_time_ms  BIGINT        NOT NULL DEFAULT 0,
    error_message       TEXT,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reviews_pull_request_id ON reviews (pull_request_id);
CREATE INDEX IF NOT EXISTS idx_reviews_status ON reviews (status);
CREATE INDEX IF NOT EXISTS idx_reviews_created_at ON reviews (created_at DESC);

-- ---------------------------------------------------------------------------
-- review_comments
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS review_comments (
    id              UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id       UUID           NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    file_path       TEXT           NOT NULL,
    line_number     INT,
    severity        severity_level NOT NULL DEFAULT 'medium',
    title           TEXT           NOT NULL,
    explanation     TEXT           NOT NULL,
    why_it_matters  TEXT,
    suggestion      TEXT,
    code_snippet    TEXT,
    published       BOOLEAN        NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_review_comments_review_id ON review_comments (review_id);
CREATE INDEX IF NOT EXISTS idx_review_comments_severity ON review_comments (severity);
CREATE INDEX IF NOT EXISTS idx_review_comments_file_path ON review_comments (file_path);

-- ---------------------------------------------------------------------------
-- execution_logs
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS execution_logs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id    UUID        REFERENCES reviews(id) ON DELETE SET NULL,
    level        VARCHAR(20) NOT NULL DEFAULT 'info',
    message      TEXT        NOT NULL,
    metadata     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_execution_logs_review_id ON execution_logs (review_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_created_at ON execution_logs (created_at DESC);

-- ---------------------------------------------------------------------------
-- updated_at trigger (auto-update timestamp on row change)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS set_repositories_updated_at ON repositories;
CREATE TRIGGER set_repositories_updated_at
    BEFORE UPDATE ON repositories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS set_pull_requests_updated_at ON pull_requests;
CREATE TRIGGER set_pull_requests_updated_at
    BEFORE UPDATE ON pull_requests
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
