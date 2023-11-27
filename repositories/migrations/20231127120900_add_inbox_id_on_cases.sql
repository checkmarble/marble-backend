-- +goose Up
-- +goose StatementBegin

ALTER TABLE cases ADD COLUMN inbox_id UUID REFERENCES inboxes ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE cases DROP COLUMN inbox_id;

-- +goose StatementEnd
