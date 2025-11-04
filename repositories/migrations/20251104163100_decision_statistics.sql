-- +goose Up
-- +goose StatementBegin
ALTER TABLE decision_rules
ALTER COLUMN decision_id
SET
    STATISTICS 10000;

ANALYZE decision_rules;

CREATE STATISTICS IF NOT EXISTS decision_org_object_time ON org_id,
trigger_object_type,
created_at
FROM
    decisions;

ANALYZE decisions;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decision_rules
ALTER COLUMN decision_id
SET
    STATISTICS -1;

ANALYZE decision_rules;

DROP STATISTICS IF EXISTS decision_org_object_time;

ANALYZE decisions;

-- +goose StatementEnd