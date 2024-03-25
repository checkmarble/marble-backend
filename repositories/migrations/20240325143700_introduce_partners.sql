-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS
      partners (
            id uuid DEFAULT uuid_generate_v4 () PRIMARY KEY,
            created_at timestamp with time zone DEFAULT NOW() NOT NULL,
            name varchar(255) NOT NULL
      );

ALTER TABLE api_keys
ADD COLUMN partner_id uuid REFERENCES partners (id) ON DELETE SET NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE api_keys
DROP COLUMN partner_id;

DROP TABLE partners;

-- +goose StatementEnd