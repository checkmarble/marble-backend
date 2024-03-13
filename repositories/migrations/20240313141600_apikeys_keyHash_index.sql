-- +goose Up
-- +goose StatementBegin
ALTER TABLE apikeys
ALTER COLUMN key_hash
SET NOT NULL;

CREATE UNIQUE INDEX apikeys_key_hash_index ON apikeys (key_hash)
WHERE
      deleted_at IS NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX apikeys_key_hash_index;

ALTER TABLE apikeys
ALTER COLUMN key_hash
DROP NOT NULL;

-- +goose StatementEnd