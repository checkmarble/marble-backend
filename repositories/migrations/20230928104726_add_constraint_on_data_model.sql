-- +goose Up
-- +goose StatementBegin

ALTER TABLE data_model_tables
ADD CONSTRAINT unique_data_model_tables_name
UNIQUE (organization_id, name);

ALTER TABLE data_model_fields
ADD CONSTRAINT unique_data_model_fields_name
UNIQUE (table_id, name);

ALTER TABLE data_model_links
ADD CONSTRAINT unique_data_model_links
UNIQUE (parent_table_id, parent_field_id, child_table_id, child_field_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE data_model_tables DROP CONSTRAINT unique_data_model_tables_name;
ALTER TABLE data_model_fields DROP CONSTRAINT unique_data_model_fields_name;
ALTER TABLE data_model_links DROP CONSTRAINT unique_data_model_links;

-- +goose StatementEnd
