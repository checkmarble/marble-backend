-- +goose Up
-- +goose StatementBegin
CREATE INDEX phantom_decisions_org_idx ON phantom_decisions (org_id, created_at DESC);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX phantom_decisions_org_idx;

-- +goose StatementEnd