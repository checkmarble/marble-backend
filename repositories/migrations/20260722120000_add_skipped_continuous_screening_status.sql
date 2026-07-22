-- +goose Up
-- +goose StatementBegin

ALTER TABLE continuous_screening_update_jobs
DROP CONSTRAINT IF EXISTS continuous_screening_update_jobs_status_check;

-- NOT VALID skips scanning existing rows, so adding the constraint only takes a brief ACCESS EXCLUSIVE lock
ALTER TABLE continuous_screening_update_jobs
ADD CONSTRAINT continuous_screening_update_jobs_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')) NOT VALID;

-- VALIDATE CONSTRAINT scans existing rows under a SHARE UPDATE EXCLUSIVE lock, which doesn't block reads/writes
ALTER TABLE continuous_screening_update_jobs
VALIDATE CONSTRAINT continuous_screening_update_jobs_status_check;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Replace `skipped` status to `pending` to not block the restoration of the constraint
UPDATE continuous_screening_update_jobs
SET status = 'pending'
WHERE status = 'skipped';

ALTER TABLE continuous_screening_update_jobs
DROP CONSTRAINT IF EXISTS continuous_screening_update_jobs_status_check;

-- NOT VALID skips scanning existing rows, so adding the constraint only takes a brief ACCESS EXCLUSIVE lock
ALTER TABLE continuous_screening_update_jobs
ADD CONSTRAINT continuous_screening_update_jobs_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed')) NOT VALID;

-- VALIDATE CONSTRAINT scans existing rows under a SHARE UPDATE EXCLUSIVE lock, which doesn't block reads/writes
ALTER TABLE continuous_screening_update_jobs
VALIDATE CONSTRAINT continuous_screening_update_jobs_status_check;

-- +goose StatementEnd
