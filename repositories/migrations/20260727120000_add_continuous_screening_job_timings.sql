-- +goose Up
-- +goose StatementBegin

-- started_at / finished_at record when the worker actually began and ended the job, which
-- created_at (enqueue time) and updated_at (bumped on every status write) cannot express.
ALTER TABLE continuous_screening_update_jobs
ADD COLUMN started_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN finished_at TIMESTAMP WITH TIME ZONE;

-- Backfill: created_at over-estimates the start by however long the job sat queued, but it is the
-- only signal left for rows that already reached a terminal status. finished_at is only meaningful
-- for terminal rows: for a row still in 'processing', updated_at is the moment it *started*, so
-- writing it as an end would make an in-flight job render as finished.
UPDATE continuous_screening_update_jobs
SET started_at = created_at,
    finished_at = CASE WHEN status IN ('completed', 'failed', 'skipped') THEN updated_at END
WHERE status <> 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE continuous_screening_update_jobs
DROP COLUMN started_at,
DROP COLUMN finished_at;

-- +goose StatementEnd
