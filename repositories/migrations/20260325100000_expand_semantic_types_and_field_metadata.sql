-- +goose Up
ALTER TABLE data_model_fields ADD COLUMN metadata jsonb DEFAULT NULL;
ALTER TABLE data_model_fields ADD COLUMN alias text NOT NULL DEFAULT '';

ALTER TABLE data_model_tables ADD COLUMN metadata jsonb DEFAULT NULL;

-- +goose Down
ALTER TABLE data_model_tables DROP COLUMN metadata;

ALTER TABLE data_model_fields DROP COLUMN metadata;
ALTER TABLE data_model_fields DROP COLUMN alias;
