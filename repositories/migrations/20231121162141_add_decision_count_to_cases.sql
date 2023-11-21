-- +goose Up
-- +goose StatementBegin
ALTER TABLE cases ADD COLUMN decisions_count INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE cases DROP COLUMN decisions_count;
-- +goose StatementEnd
