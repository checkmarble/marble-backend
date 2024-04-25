-- +goose Up
-- +goose StatementBegin
CREATE INDEX decisions_pivot_value_index ON decisions (org_id, pivot_value, created_at DESC);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_pivot_value_index;

-- +goose StatementEnd