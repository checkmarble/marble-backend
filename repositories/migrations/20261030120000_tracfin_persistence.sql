-- +goose Up
CREATE TABLE tracfin_configuration (
    org_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    settings jsonb NOT NULL,
    mappings jsonb NOT NULL,
    profile_version text NOT NULL,
    profile_hash text NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid NOT NULL REFERENCES users(id)
);

CREATE TABLE tracfin_drafts (
    case_id uuid PRIMARY KEY REFERENCES cases(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    content jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    last_export jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid NOT NULL REFERENCES users(id)
);
CREATE INDEX tracfin_drafts_org_case_idx ON tracfin_drafts(org_id, case_id);

-- +goose Down
DROP TABLE tracfin_drafts;
DROP TABLE tracfin_configuration;
