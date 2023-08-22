-- +goose Up
-- +goose StatementBegin
-- add default role to tokens
-- default is API_CLIENT=5
ALTER TABLE tokens RENAME TO apikeys;
ALTER TABLE apikeys RENAME COLUMN Token TO key;
ALTER TABLE apikeys ADD COLUMN role INTEGER NOT NULL DEFAULT 5;
CREATE UNIQUE INDEX apikey_key_idx ON apikeys(key);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX apikey_key_idx;
ALTER TABLE apiKeys DROP COLUMN role;
ALTER TABLE apikeys RENAME COLUMN key TO Token;
ALTER TABLE apikeys RENAME TO tokens;

-- +goose StatementEnd
