-- 001_initial_schema.sql
-- Initial database schema for CodePilot AI

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Custom enum types
DO $$ BEGIN
    CREATE TYPE review_status AS ENUM ('pending', 'in_progress', 'completed', 'failed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE severity_level AS ENUM ('critical', 'high', 'medium', 'low');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE pr_state AS ENUM ('open', 'closed', 'merged');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Repositories table
CREATE TABLE IF NOT EXISTS repositories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    github_id BIGINT UNIQUE NOT NULL,
    owner VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    full_name VARCHAR(512) UNIQUE NOT NULL,
    description TEXT,
    default_branch VARCHAR(255) DEFAULT 'main',
    language VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    webhook_id BIGINT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Pull requests table
CREATE TABLE IF NOT EXISTS pull_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    repository_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    github_number INTEGER NOT NULL,
    title VARCHAR(1024) NOT NULL,
    body TEXT,
    state pr_state DEFAULT 'open',
    author VARCHAR(255) NOT NULL,
    head_branch VARCHAR(255),
    base_branch VARCHAR(255),
    head_sha VARCHAR(64),
    additions INTEGER DEFAULT 0,
    deletions INTEGER DEFAULT 0,
    changed_files INTEGER DEFAULT 0,
    github_url VARCHAR(2048),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(repository_id, github_number)
);

-- Reviews table
CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pull_request_id UUID REFERENCES pull_requests(id) ON DELETE CASCADE,
    status review_status DEFAULT 'pending',
    summary TEXT,
    total_comments INTEGER DEFAULT 0,
    critical_count INTEGER DEFAULT 0,
    high_count INTEGER DEFAULT 0,
    medium_count INTEGER DEFAULT 0,
    low_count INTEGER DEFAULT 0,
    llm_model VARCHAR(255),
    tokens_used INTEGER DEFAULT 0,
    processing_time_ms BIGINT DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Review comments table
CREATE TABLE IF NOT EXISTS review_comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id UUID REFERENCES reviews(id) ON DELETE CASCADE,
    file_path VARCHAR(1024) NOT NULL,
    line_number INTEGER,
    severity severity_level NOT NULL,
    title VARCHAR(512) NOT NULL,
    explanation TEXT NOT NULL,
    why_it_matters TEXT,
    suggestion TEXT,
    code_snippet TEXT,
    published BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Execution logs table
CREATE TABLE IF NOT EXISTS execution_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id UUID REFERENCES reviews(id) ON DELETE CASCADE,
    step VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    message TEXT,
    metadata JSONB DEFAULT '{}',
    duration_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_pull_requests_repository ON pull_requests(repository_id);
CREATE INDEX IF NOT EXISTS idx_reviews_pull_request ON reviews(pull_request_id);
CREATE INDEX IF NOT EXISTS idx_review_comments_review ON review_comments(review_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_review ON execution_logs(review_id);
CREATE INDEX IF NOT EXISTS idx_repositories_full_name ON repositories(full_name);
