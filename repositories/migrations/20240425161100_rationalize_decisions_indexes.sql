-- +goose Up
-- +goose StatementBegin
CREATE INDEX decisions_by_org_id_index ON decisions (org_id, created_at DESC) INCLUDE (scenario_id, outcome, trigger_object_type, case_id);

CREATE INDEX decisions_scheduled_execution_id_idx_2 ON decisions (org_id, scheduled_execution_id, created_at DESC);

DROP INDEX decisions_org_id_idx;

DROP INDEX decisions_scheduled_execution_id_idx;

DROP INDEX scenario_id_idx;

DROP INDEX outcome_idx;

DROP INDEX trigger_object_type_idx;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
CREATE INDEX trigger_object_type_idx ON decisions (org_id, trigger_object_type, created_at DESC);

CREATE INDEX outcome_idx ON decisions (org_id, outcome, created_at DESC);

CREATE INDEX scenario_id_idx ON decisions (org_id, scenario_id, created_at DESC);

CREATE INDEX decisions_scheduled_execution_id_idx ON decisions (scheduled_execution_id, created_at DESC);

CREATE INDEX decisions_org_id_idx ON decisions (org_id, created_at DESC);

DROP INDEX decisions_pivot_value_index;

DROP INDEX decisions_by_org_id_index;

-- +goose StatementEnd