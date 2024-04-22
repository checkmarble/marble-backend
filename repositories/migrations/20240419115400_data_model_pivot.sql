-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS
      data_model_pivots (
            id uuid DEFAULT uuid_generate_v4 () PRIMARY KEY,
            base_table_id uuid NOT NULL REFERENCES data_model_tables (id) ON DELETE CASCADE,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
            field_id uuid REFERENCES data_model_fields (id) ON DELETE CASCADE,
            organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
            path_link_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[]
      );

CREATE UNIQUE INDEX data_model_pivots_base_table_id_idx ON data_model_pivots (organization_id, base_table_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX data_model_pivots_base_table_id_idx;

DROP TABLE data_model_pivots;

-- +goose StatementEnd