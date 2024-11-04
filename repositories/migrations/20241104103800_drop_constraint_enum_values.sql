-- +goose Up
-- +goose StatementBegin
ALTER TABLE data_model_enum_values
DROP CONSTRAINT IF EXISTS data_model_enum_values_field_id_fkey;

ALTER TABLE data_model_enum_values
DROP COLUMN last_seen;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE data_model_enum_values
ADD CONSTRAINT data_model_enum_values_field_id_fkey FOREIGN KEY (field_id) REFERENCES data_model_fields (id) ON DELETE CASCADE;

ALTER TABLE data_model_enum_values
ADD COLUMN last_seen TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW();

-- +goose StatementEnd