-- +goose Up
-- +goose StatementBegin
--- those views are created outside if this migration file, but we need to drop it here becauses it uses the table name
-- (it wil be recreated as the migrations script is run)
DROP VIEW IF EXISTS analytics.apikeys;

DROP INDEX apikey_key_idx;

UPDATE apikeys
SET
      key = SUBSTRING(key, 1, 3)
where
      true;

ALTER TABLE apikeys
RENAME COLUMN key TO prefix;

ALTER TABLE apikeys
RENAME TO api_keys;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE api_keys
RENAME TO apikeys;

ALTER TABLE apikeys
RENAME COLUMN prefix TO key;

CREATE UNIQUE INDEX apikey_key_idx ON apikeys (prefixkey);

-- +goose StatementEnd