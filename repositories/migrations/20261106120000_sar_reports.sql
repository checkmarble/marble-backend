-- +goose Up
-- Format-neutral SAR pivot per case, reports per (format, country, version),
-- traceable revisions, agent runs, per-format error memory and correction chat.
CREATE TABLE sar_configurations (
 org_id uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
 revision bigint NOT NULL DEFAULT 1,
 content jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sar_pivots (
 case_id uuid PRIMARY KEY REFERENCES cases(id) ON DELETE CASCADE,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 revision bigint NOT NULL DEFAULT 1,
 content jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sar_pivots_org ON sar_pivots(org_id);
CREATE TABLE sar_reports (
 id uuid PRIMARY KEY,
 case_id uuid NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 format_key text NOT NULL,
 revision bigint NOT NULL DEFAULT 1,
 content jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (case_id, format_key)
);
CREATE INDEX sar_reports_org_case ON sar_reports(org_id, case_id);
CREATE TABLE sar_revisions (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 case_id uuid NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
 object text NOT NULL,
 revision bigint NOT NULL,
 origin text NOT NULL,
 actor_id text NOT NULL,
 run_id text NOT NULL DEFAULT '',
 chat_id text NOT NULL DEFAULT '',
 summary text NOT NULL,
 patch jsonb,
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sar_revisions_case ON sar_revisions(org_id, case_id, object, revision DESC);
CREATE TABLE sar_runs (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 case_id uuid NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
 status text NOT NULL,
 content jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sar_runs_case ON sar_runs(org_id, case_id, created_at DESC);
CREATE UNIQUE INDEX sar_runs_active_case ON sar_runs(case_id) WHERE status IN ('queued','running');
CREATE TABLE sar_format_memory (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 format_key text NOT NULL,
 code text NOT NULL,
 path text NOT NULL,
 cause text NOT NULL,
 fix text NOT NULL DEFAULT '',
 frequency int NOT NULL DEFAULT 1,
 corrected int NOT NULL DEFAULT 0,
 first_seen timestamptz NOT NULL DEFAULT now(),
 last_seen timestamptz NOT NULL DEFAULT now(),
 UNIQUE (org_id, format_key, code, path)
);
CREATE TABLE sar_chat_messages (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 case_id uuid NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
 report_id uuid NOT NULL REFERENCES sar_reports(id) ON DELETE CASCADE,
 content jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sar_chat_report ON sar_chat_messages(report_id, created_at);
-- +goose Down
DROP TABLE sar_chat_messages;
DROP TABLE sar_format_memory;
DROP TABLE sar_runs;
DROP TABLE sar_revisions;
DROP TABLE sar_reports;
DROP TABLE sar_pivots;
DROP TABLE sar_configurations;
