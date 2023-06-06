-- +goose Up
-- +goose StatementBegin

CREATE TABLE client_tables (
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid,
  -- TODO: add cloudSqlInstanceId. As of today, all clients are stored in marble db.
  schema_name VARCHAR(255) NOT NULL,
  PRIMARY KEY(id),
  CONSTRAINT fk_client_tables_organization FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX client_tables_org_id_idx ON client_tables(org_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX client_tables_org_id_idx;
DROP TABLE client_tables;
-- +goose StatementEnd
