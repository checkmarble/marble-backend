-- +goose Up
-- +goose StatementBegin
ALTER TABLE apikeys
ADD COLUMN key_hash bytea;

ALTER TABLE apikeys
ADD COLUMN created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now();

UPDATE apikeys
SET
      key_hash = sha256(key::bytea)
where
      key_hash is null;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE apikeys
DROP COLUMN key_hash;

ALTER TABLE apikeys
DROP COLUMN created_at;

-- +goose StatementEnd