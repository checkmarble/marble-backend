-- +goose NO TRANSACTION
-- +goose Up 
CREATE INDEX CONCURRENTLY IF NOT EXISTS decision_rules_org_id_idx ON decision_rules (org_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS decision_pivot_id_idx ON decisions (pivot_id);

-- +goose Down
DROP INDEX decision_rules_org_id_idx;

DROP INDEX decision_pivot_id_idx;

-- +goose StatementBegin
-- +goose StatementEnd