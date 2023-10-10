-- +goose Up
-- +goose StatementBegin

ALTER TABLE data_model_fields
ADD COLUMN is_enum BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE data_model_enum_values (
    field_id    UUID REFERENCES data_model_fields ON DELETE CASCADE NOT NULL,
    value       TEXT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_seen   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE data_model_enum_values
ADD CONSTRAINT unique_data_model_enum_values_field_id_value
UNIQUE (field_id, value);

CREATE INDEX data_model_enum_values_field_id_last_seen ON data_model_enum_values(field_id, last_seen DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE data_model_fields DROP COLUMN is_enum;
DROP TABLE data_model_enum_values;

-- +goose StatementEnd
