-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    org_id UUID,
    key TEXT NOT NULL,
    value TEXT NOT NULL,

    CONSTRAINT uq_metadata_org_id_key UNIQUE NULLS NOT DISTINCT (org_id, key),
    CONSTRAINT fk_metadata_org_id
        FOREIGN KEY (org_id) REFERENCES organizations (id)
        ON DELETE CASCADE
);

INSERT INTO metadata (key, value) VALUES ('deployment_id', gen_random_uuid()) ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE metadata;

-- +goose StatementEnd
