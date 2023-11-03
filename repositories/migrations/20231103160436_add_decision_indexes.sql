-- +goose Up
-- +goose StatementBegin
DROP INDEX scenario_id_idx;
DROP INDEX created_at_idx;
DROP INDEX outcome_idx;
DROP INDEX trigger_object_type_idx;
CREATE INDEX scenario_id_idx ON decisions(org_id, scenario_id, created_at DESC);
CREATE INDEX outcome_idx ON decisions(org_id, outcome, created_at DESC);
CREATE INDEX trigger_object_type_idx ON decisions(org_id, trigger_object_type, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX scenario_id_idx;
DROP INDEX outcome_idx;
DROP INDEX trigger_object_type_idx;
-- +goose StatementEnd
