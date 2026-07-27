-- +goose Up
ALTER TABLE scenario_iteration_rules
    ADD COLUMN ai_description text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE scenario_iteration_rules
    DROP COLUMN ai_description;
