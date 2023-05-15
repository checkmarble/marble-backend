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

CREATE TABLE bank_accounts(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  company_id VARCHAR,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
  valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
  balance double precision,
  name VARCHAR,
  currency VARCHAR NOT NULL,
  PRIMARY KEY(id)
);
CREATE INDEX bank_accounts_object_id_idx ON bank_accounts(object_id, valid_until DESC, valid_from, updated_at);

CREATE TABLE transactions(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
  valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
  value double precision,
  title VARCHAR,
  description VARCHAR,
  bank_account_id VARCHAR,
  PRIMARY KEY(id)
);
CREATE INDEX transactions_object_id_idx ON transactions(object_id, valid_until DESC, valid_from, updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX companies_object_id_idx;
DROP TABLE companies;
DROP INDEX bank_accounts_object_id_idx;
DROP TABLE bank_accounts;
DROP INDEX transactions_object_id_idx;
DROP TABLE transactions;
-- +goose StatementEnd
