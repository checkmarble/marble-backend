-- +goose Up
-- +goose StatementBegin
CREATE INDEX decisions_org_id_idx ON decisions(org_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_org_id_idx;
-- +goose StatementEnd
