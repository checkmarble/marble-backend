-- +goose Up
-- +goose StatementBegin

ALTER TABLE data_model_enum_values ALTER COLUMN value DROP NOT NULL;

ALTER TABLE data_model_enum_values ADD COLUMN float_value FLOAT;
ALTER TABLE data_model_enum_values ADD COLUMN text_value TEXT;

ALTER TABLE data_model_enum_values
ADD CONSTRAINT unique_data_model_enum_float_values_field_id_value UNIQUE (field_id, float_value);

ALTER TABLE data_model_enum_values
ADD CONSTRAINT unique_data_model_enum_text_values_field_id_value UNIQUE (field_id, text_value);

ALTER TABLE data_model_enum_values DROP CONSTRAINT unique_data_model_enum_values_field_id_value

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE data_model_enum_values ALTER COLUMN value SET NOT NULL;
ALTER TABLE data_model_enum_values DROP COLUMN text_value;
ALTER TABLE data_model_enum_values DROP COLUMN float_value;
ALTER TABLE data_model_enum_values DROP CONSTRAINT IF EXISTS unique_data_model_enum_float_values_field_id_value;
ALTER TABLE data_model_enum_values DROP CONSTRAINT IF EXISTS unique_data_model_enum_text_values_field_id_value;

ALTER TABLE data_model_enum_values
ADD CONSTRAINT unique_data_model_enum_values_field_id_value UNIQUE (field_id, value);

-- +goose StatementEnd
