-- +goose Up
-- +goose StatementBegin
ALTER TABLE apikeys ADD COLUMN description VARCHAR(255) NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE apikeys DROP COLUMN description;
-- +goose StatementEnd
