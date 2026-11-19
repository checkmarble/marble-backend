-- +goose Up
-- +goose StatementBegin
-- auto_added: the server added the object as a parent of another selected
-- object (display). context_only: it is context to read and cite, never
-- declared on its own (false for a parent that is also a direct candidate).
ALTER TABLE sar_declaration_selections
    ADD COLUMN auto_added boolean NOT NULL DEFAULT false,
    ADD COLUMN context_only boolean NOT NULL DEFAULT false;

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
    DROP COLUMN context_only,
    DROP COLUMN auto_added;
-- +goose StatementEnd
