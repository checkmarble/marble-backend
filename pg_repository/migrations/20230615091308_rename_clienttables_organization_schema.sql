-- +goose Up
-- +goose StatementBegin

ALTER TABLE client_tables RENAME TO organizations_schema;
ALTER INDEX client_tables_org_id_idx RENAME TO organization_schema_org_id_idx;
ALTER TABLE organizations_schema RENAME CONSTRAINT fk_client_tables_organization TO fk_organization_schema_organization;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations_schema RENAME CONSTRAINT fk_organization_schema_organization TO fk_client_tables_organization;
ALTER INDEX organization_schema_org_id_idx RENAME TO client_tables_org_id_idx;
ALTER TABLE organizations_schema RENAME TO client_tables;

-- +goose StatementEnd
