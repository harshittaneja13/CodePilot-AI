-- Add columns to execution_logs that the review engine requires but were
-- missing from the initial schema.
ALTER TABLE execution_logs
    ADD COLUMN IF NOT EXISTS step        VARCHAR(100),
    ADD COLUMN IF NOT EXISTS status      VARCHAR(20) DEFAULT 'info',
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT      DEFAULT 0;
