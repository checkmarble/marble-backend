-- +goose Up
CREATE TABLE tracfin_ai_runs (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 case_id uuid NOT NULL REFERENCES tracfin_drafts(case_id) ON DELETE CASCADE,
 actor_id uuid NOT NULL REFERENCES users(id),
 status text NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 content jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tracfin_ai_active_case ON tracfin_ai_runs(case_id) WHERE status IN ('queued','running');
CREATE INDEX tracfin_ai_case_runs ON tracfin_ai_runs(org_id,case_id,created_at DESC);
-- +goose Down
DROP TABLE tracfin_ai_runs;
