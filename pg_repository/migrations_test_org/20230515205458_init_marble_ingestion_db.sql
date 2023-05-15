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

CREATE TABLE accountss(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
  valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
  balance double precision,
  company_id VARCHAR,
  currency VARCHAR NOT NULL,
  is_frozen BOOLEAN,
  name VARCHAR,
  PRIMARY KEY(id)
);
CREATE INDEX accountss_object_id_idx ON accountss(object_id, valid_until DESC, valid_from, updated_at);

CREATE TABLE transactions(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
  valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',
  amount double precision,
  accounts_id VARCHAR,
  bic_country VARCHAR,
  country VARCHAR,
  description VARCHAR,
  direction VARCHAR,
  status VARCHAR,
  title VARCHAR,
  PRIMARY KEY(id)
);
CREATE INDEX transactions_object_id_idx ON transactions(object_id, valid_until DESC, valid_from, updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX companies_object_id_idx;
DROP TABLE companies;
DROP INDEX accountss_object_id_idx;
DROP TABLE accountss;
DROP INDEX transactions_object_id_idx;
DROP TABLE transactions;
-- +goose StatementEnd
