-- +goose Up
ALTER TABLE data_model_fields ADD COLUMN semantic_type text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE data_model_fields DROP COLUMN semantic_type;