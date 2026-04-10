-- +goose Up
-- +goose StatementBegin

-- 1. Migrate displayed_fields → field metadata {"hidden": true}
-- For each table with data_model_options, fields NOT in displayed_fields get hidden=true.
-- Only apply when displayed_fields is not empty (empty means no preference was set).
-- object_id is excluded because it is never part of displayed_fields/field_order options.
UPDATE data_model_fields f
SET metadata = COALESCE(f.metadata, '{}'::jsonb) || '{"hidden": true}'::jsonb
FROM data_model_options o
WHERE f.table_id = o.table_id
  AND array_length(o.displayed_fields, 1) IS NOT NULL
  AND f.id != ALL(o.displayed_fields)
  AND f.name != 'object_id';

-- 2. Migrate field_order → table metadata {"fieldOrder": ["field_name_1", ...]}
-- Convert UUIDs to field names using a subquery, preserving the order.
UPDATE data_model_tables t
SET metadata = COALESCE(t.metadata, '{}'::jsonb) || jsonb_build_object('fieldOrder', ordered_names.names)
FROM data_model_options o
CROSS JOIN LATERAL (
    SELECT COALESCE(array_agg(f.name ORDER BY idx.ord), '{}') AS names
    FROM unnest(o.field_order) WITH ORDINALITY AS idx(field_id, ord)
    JOIN data_model_fields f ON f.id = idx.field_id
) ordered_names
WHERE t.id = o.table_id
  AND array_length(o.field_order, 1) IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- no downgrade
