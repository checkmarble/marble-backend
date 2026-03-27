-- +goose Up
ALTER TABLE data_model_fields
    ADD COLUMN semantic_type text NOT NULL DEFAULT '',
    ADD COLUMN is_primary_ordering boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT is_primary_ordering_only_for_timestamp
        CHECK (is_primary_ordering = false OR type = 'Timestamp');

-- At most one field per table can be the primary ordering field
CREATE UNIQUE INDEX data_model_fields_one_primary_ordering_per_table
    ON data_model_fields (table_id) WHERE is_primary_ordering = true;

-- +goose Down
DROP INDEX data_model_fields_one_primary_ordering_per_table;

ALTER TABLE data_model_fields
    DROP CONSTRAINT is_primary_ordering_only_for_timestamp,
    DROP COLUMN semantic_type,
    DROP COLUMN is_primary_ordering;
