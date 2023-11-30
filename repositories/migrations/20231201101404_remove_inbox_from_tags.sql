-- +goose Up
-- +goose StatementBegin
ALTER TABLE tags DROP COLUMN inbox_id;
ALTER TABLE tags ADD COLUMN org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS tags_unique_name_inbox_id;
CREATE UNIQUE INDEX tags_unique_name_org_id ON tags (name, org_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX tags_unique_name_org_id;
ALTER TABLE tags DROP COLUMN org_id;
ALTER TABLE tags ADD COLUMN inbox_id UUID NOT NULL REFERENCES inboxes(id) ON DELETE CASCADE;
CREATE UNIQUE INDEX tags_unique_name_inbox_id ON tags (name, inbox_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd
