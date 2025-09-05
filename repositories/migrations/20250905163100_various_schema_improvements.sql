-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
SET
    (autovacuum_vacuum_insert_scale_factor = 0.02);

ALTER TABLE decision_rules
SET
    (autovacuum_vacuum_insert_scale_factor = 0.02);

ALTER TABLE decisions
SET
    (autovacuum_analyze_scale_factor = 0.01);

ALTER TABLE decision_rules
SET
    (autovacuum_analyze_scale_factor = 0.01);

ALTER TABLE webhook_events
SET
    (autovacuum_vacuum_insert_scale_factor = 0.02);

ALTER TABLE webhook_events
SET
    (autovacuum_analyze_scale_factor = 0.01);

ALTER TABLE phantom_decisions
SET
    (autovacuum_vacuum_insert_scale_factor = 0.02);

ALTER TABLE phantom_decisions
SET
    (autovacuum_analyze_scale_factor = 0.01);

ALTER TABLE sanction_checks
SET
    (autovacuum_vacuum_insert_scale_factor = 0.02);

ALTER TABLE sanction_checks
SET
    (autovacuum_analyze_scale_factor = 0.01);

ALTER TABLE decisions_to_create
SET
    (autovacuum_vacuum_insert_scale_factor = 0.02);

ALTER TABLE decisions_to_create
SET
    (autovacuum_analyze_scale_factor = 0.01);

ALTER TABLE decisions
ALTER COLUMN scenario_name
DROP NOT NULL,
ALTER COLUMN scenario_description
DROP NOT NULL,
ALTER COLUMN scenario_version
DROP NOT NULL;

-- remove FK on scenario_iteration_id on decisions
ALTER TABLE decisions
DROP CONSTRAINT decisions_scenario_iteration_id_fkey;

-- and on pivot_id
ALTER TABLE decisions
DROP CONSTRAINT decisions_pivot_id_fkey;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decisions
RESET (autovacuum_vacuum_insert_scale_factor);

ALTER TABLE decision_rules
RESET (autovacuum_vacuum_insert_scale_factor);

ALTER TABLE decisions
RESET (autovacuum_analyze_scale_factor);

ALTER TABLE decision_rules
RESET (autovacuum_analyze_scale_factor);

ALTER TABLE webhook_events
RESET (autovacuum_vacuum_insert_scale_factor);

ALTER TABLE webhook_events
RESET (autovacuum_analyze_scale_factor);

ALTER TABLE phantom_decisions
RESET (autovacuum_vacuum_insert_scale_factor);

ALTER TABLE phantom_decisions
RESET (autovacuum_analyze_scale_factor);

ALTER TABLE sanction_checks
RESET (autovacuum_vacuum_insert_scale_factor);

ALTER TABLE sanction_checks
RESET (autovacuum_analyze_scale_factor);

ALTER TABLE decisions_to_create
RESET (autovacuum_vacuum_insert_scale_factor);

ALTER TABLE decisions_to_create
RESET (autovacuum_analyze_scale_factor);

ALTER TABLE decisions
ALTER COLUMN scenario_name
SET NOT NULL,
ALTER COLUMN scenario_description
SET NOT NULL,
ALTER COLUMN scenario_version
SET NOT NULL;

ALTER TABLE decisions
ADD CONSTRAINT decisions_scenario_iteration_id_fkey FOREIGN KEY (scenario_iteration_id) REFERENCES scenario_iterations (id);

ALTER TABLE decisions
ADD CONSTRAINT decisions_pivot_id_fkey FOREIGN KEY (pivot_id) REFERENCES data_model_pivots (id);

-- +goose StatementEnd