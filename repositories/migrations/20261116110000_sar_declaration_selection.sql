-- +goose Up
-- +goose StatementBegin
CREATE TABLE sar_declaration_selections (
    id uuid PRIMARY KEY,
    declaration_id uuid NOT NULL REFERENCES sar_declarations(id) ON DELETE CASCADE,
    table_name text NOT NULL,
    object_id text NOT NULL,
    snapshot jsonb NOT NULL,
    origin jsonb NOT NULL,
    reached_from jsonb,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT sar_declaration_selections_object_unique UNIQUE (declaration_id, table_name, object_id)
);

CREATE INDEX sar_declaration_selections_declaration_idx ON sar_declaration_selections (declaration_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sar_declaration_selections;
-- +goose StatementEnd
