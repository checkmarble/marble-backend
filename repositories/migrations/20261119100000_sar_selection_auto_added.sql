-- +goose Up
-- +goose StatementBegin
ALTER TABLE sar_declaration_selections
    ADD COLUMN auto_added boolean NOT NULL DEFAULT false;

-- Objects the analyst removed although the server had added them on its own:
-- later additions must not bring them back.
CREATE TABLE sar_declaration_selection_exclusions (
    declaration_id uuid NOT NULL REFERENCES sar_declarations(id) ON DELETE CASCADE,
    table_name text NOT NULL,
    object_id text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (declaration_id, table_name, object_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sar_declaration_selection_exclusions;

ALTER TABLE sar_declaration_selections
    DROP COLUMN auto_added;
-- +goose StatementEnd
