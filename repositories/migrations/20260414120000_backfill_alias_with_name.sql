-- +goose Up
UPDATE data_model_tables SET alias = name WHERE alias = '';
UPDATE data_model_fields SET alias = name WHERE alias = '';

-- +goose Down
-- No rollback
