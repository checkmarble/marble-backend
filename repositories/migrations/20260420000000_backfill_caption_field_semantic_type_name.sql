-- +goose Up
UPDATE data_model_fields f
SET semantic_type = 'name'
FROM data_model_tables t
WHERE f.table_id = t.id
  AND t.caption_field <> ''
  AND f.name = t.caption_field
  AND f.semantic_type = '';

-- +goose Down
-- No rollback
