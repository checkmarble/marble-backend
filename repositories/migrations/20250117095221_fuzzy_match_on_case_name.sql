-- +goose NO TRANSACTION
-- +goose Up

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose StatementBegin

DO $$
BEGIN
   EXECUTE format('ALTER ROLE %I SET pg_trgm.similarity_threshold = 0.1', current_user);
END
$$;

-- +goose StatementEnd

CREATE INDEX CONCURRENTLY trgm_cases_on_name ON cases USING GIN (name gin_trgm_ops);

CREATE INDEX CONCURRENTLY case_org_id_idx_2 ON cases (org_id, created_at DESC) INCLUDE (inbox_id, status, name);

DROP INDEX CONCURRENTLY IF EXISTS case_org_id_idx;

DROP INDEX CONCURRENTLY IF EXISTS case_status_idx;

-- +goose Down
CREATE INDEX CONCURRENTLY case_status_idx ON cases (org_id, status, created_at DESC);

CREATE INDEX CONCURRENTLY case_org_id_idx ON cases (org_id, created_at DESC);

DROP INDEX CONCURRENTLY IF EXISTS case_org_id_idx_2;

DROP INDEX CONCURRENTLY IF EXISTS trgm_cases_on_name;
