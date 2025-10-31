-- +goose Up
-- +goose StatementBegin

ALTER TABLE data_model_tables
ADD COLUMN ftm_entity TEXT;

ALTER TABLE data_model_fields
ADD COLUMN ftm_property TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE data_model_tables
DROP COLUMN ftm_entity;

ALTER TABLE data_model_fields
DROP COLUMN ftm_property;

-- +goose StatementEnd
