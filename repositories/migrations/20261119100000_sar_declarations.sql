-- +goose Up
CREATE TABLE sar_declarations (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    case_id uuid NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    format_key text NOT NULL,
    report_type text NOT NULL,
    document jsonb NOT NULL,
    version integer NOT NULL DEFAULT 1,
    analysis text NOT NULL DEFAULT '',
    extra_info text NOT NULL DEFAULT '',
    created_by uuid,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX sar_declarations_case_idx ON sar_declarations (case_id);

CREATE TABLE sar_org_format_settings (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    format_family text NOT NULL,
    "values" jsonb NOT NULL,
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, format_family)
);

CREATE TABLE sar_declaration_exports (
    id uuid PRIMARY KEY,
    declaration_id uuid NOT NULL REFERENCES sar_declarations(id) ON DELETE CASCADE,
    declaration_version integer NOT NULL,
    document jsonb NOT NULL,
    outstanding jsonb NOT NULL DEFAULT '[]',
    bucket_name text NOT NULL DEFAULT '',
    file_reference text NOT NULL DEFAULT '',
    sha256 text NOT NULL DEFAULT '',
    created_by uuid,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX sar_declaration_exports_declaration_idx ON sar_declaration_exports (declaration_id);

-- +goose Down
DROP TABLE sar_declaration_exports;
DROP TABLE sar_org_format_settings;
DROP TABLE sar_declarations;
