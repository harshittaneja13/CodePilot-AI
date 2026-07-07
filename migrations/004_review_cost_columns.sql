-- ============================================================================
-- Migration 004: per-review cost accounting
-- Adds input/output token split and computed USD cost to the reviews table.
-- Idempotent (ADD COLUMN IF NOT EXISTS) — the runner re-applies every file.
-- ============================================================================

ALTER TABLE reviews ADD COLUMN IF NOT EXISTS input_tokens  INT              NOT NULL DEFAULT 0;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS output_tokens INT              NOT NULL DEFAULT 0;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS cost_usd      DOUBLE PRECISION NOT NULL DEFAULT 0;
