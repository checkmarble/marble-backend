-- +goose Up
-- +goose StatementBegin
CREATE INDEX scenario_id_idx ON decisions(scenario_id);
CREATE INDEX created_at_idx ON decisions(created_at);
CREATE INDEX outcome_idx ON decisions(outcome);
CREATE INDEX trigger_object_type_idx ON decisions(trigger_object_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX scenario_id_idx;
DROP INDEX created_at_idx;
DROP INDEX outcome_idx;
DROP INDEX trigger_object_type_idx;
-- +goose StatementEnd
