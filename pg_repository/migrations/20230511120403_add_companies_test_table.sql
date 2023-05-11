-- +goose Up
-- +goose StatementBegin
CREATE TABLE companies(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
  valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
  name VARCHAR,
  PRIMARY KEY(id)
);

CREATE INDEX companies_object_id_idx ON companies(object_id, valid_until DESC, valid_from, updated_at);

ALTER TABLE bank_accounts ADD COLUMN company_id VARCHAR;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX companies_object_id_idx;
DROP TABLE companies;
ALTER TABLE bank_accounts DROP COLUMN company_id;
-- +goose StatementEnd