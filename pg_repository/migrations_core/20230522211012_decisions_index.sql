-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions ADD COLUMN trigger_object_type VARCHAR;
ALTER TABLE decisions ADD COLUMN trigger_object json;
CREATE INDEX decisions_org_id_idx ON decisions(org_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE decisions DROP COLUMN trigger_object_type;
ALTER TABLE decisions DROP COLUMN trigger_object;
DROP INDEX decisions_org_id_idx;
-- +goose StatementEnd
