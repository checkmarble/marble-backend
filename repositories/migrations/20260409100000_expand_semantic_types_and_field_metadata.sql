-- +goose Up
ALTER TABLE data_model_fields ADD COLUMN metadata jsonb DEFAULT NULL;
ALTER TABLE data_model_fields ADD COLUMN alias text NOT NULL DEFAULT '';

ALTER TABLE data_model_tables ADD COLUMN metadata jsonb DEFAULT NULL;
ALTER TABLE data_model_tables ADD COLUMN primary_ordering_field text NOT NULL DEFAULT '';

ALTER TABLE data_model_fields ADD COLUMN semantic_type text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE data_model_fields DROP COLUMN semantic_type;

ALTER TABLE data_model_tables DROP COLUMN primary_ordering_field;
ALTER TABLE data_model_tables DROP COLUMN metadata;

ALTER TABLE data_model_fields DROP COLUMN alias;
ALTER TABLE data_model_fields DROP COLUMN metadata;
