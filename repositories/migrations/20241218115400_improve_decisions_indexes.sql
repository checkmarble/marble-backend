-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY decisions_case_id_idx_2 ON decisions (case_id, org_id)
WHERE
      case_id IS NOT NULL;

-- This index servies to fix an annoying case, where the postgres query optimized, given filters on decisions with "org_id=... and case_id IS NOT NULL" sometimes
-- tries to use the decisions_case_id_idx index, reading all rows in the index and then ordering them in a second step. This is pretty sensitive to the distribution of 
-- values in the tables and indexes, so is a bit hard to reproduce.
CREATE INDEX CONCURRENTLY decisions_org_search_idx_with_case ON decisions (org_id, created_at DESC) INCLUDE (scenario_id, outcome, trigger_object_type, case_id, review_status)
WHERE
      case_id IS NOT NULL;

CREATE INDEX CONCURRENTLY decisions_scheduled_execution_id_idx_3 ON decisions (scheduled_execution_id, created_at DESC)
WHERE
      scheduled_execution_id IS NOT NULL;

DROP INDEX decisions_case_id_idx;

DROP INDEX decisions_scheduled_execution_id_idx_2;

-- +goose Down
CREATE INDEX CONCURRENTLY decisions_case_id_idx ON decisions (org_id, case_id) INCLUDE (pivot_value);

CREATE INDEX CONCURRENTLY decisions_scheduled_execution_id_idx_2 ON decisions (org_id, scheduled_execution_id, created_at DESC);

DROP INDEX decisions_case_id_idx_2;

DROP INDEX decisions_org_search_idx_with_case;

DROP INDEX decisions_scheduled_execution_id_idx_3;