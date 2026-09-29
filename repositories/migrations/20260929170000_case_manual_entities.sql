-- +goose Up
CREATE TABLE case_manual_entities (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    case_id uuid NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    table_name text NOT NULL,
    object_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT case_manual_entities_unique_ref UNIQUE (case_id, table_name, object_id)
);

CREATE INDEX case_manual_entities_object_idx
    ON case_manual_entities (org_id, table_name, object_id, case_id);

-- +goose Down
DROP TABLE case_manual_entities;
