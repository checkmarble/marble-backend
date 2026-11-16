-- +goose Up
-- +goose StatementBegin
CREATE TABLE sar_generation_jobs (
    id uuid PRIMARY KEY,
    declaration_id uuid NOT NULL REFERENCES sar_declarations(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('generate', 'review')),
    status text NOT NULL CHECK (status IN ('pending', 'running', 'done', 'failed')),
    input jsonb,
    error text,
    created_by uuid,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

-- At most one active job per declaration.
CREATE UNIQUE INDEX sar_generation_jobs_one_active_idx
    ON sar_generation_jobs (declaration_id)
    WHERE status IN ('pending', 'running');

CREATE TABLE sar_generation_tasks (
    id uuid PRIMARY KEY,
    job_id uuid NOT NULL REFERENCES sar_generation_jobs(id) ON DELETE CASCADE,
    kind text NOT NULL,
    target_path text,
    input jsonb NOT NULL,
    input_hash text NOT NULL,
    depends_on uuid[] NOT NULL DEFAULT '{}',
    status text NOT NULL CHECK (status IN ('pending', 'running', 'done', 'failed')),
    attempts int NOT NULL DEFAULT 0,
    output jsonb,
    error text,
    trace jsonb,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX sar_generation_tasks_job_id_idx ON sar_generation_tasks (job_id);

CREATE TABLE sar_declaration_questions (
    id uuid PRIMARY KEY,
    declaration_id uuid NOT NULL REFERENCES sar_declarations(id) ON DELETE CASCADE,
    task_id uuid,
    kind text NOT NULL CHECK (kind IN ('question', 'review_finding')),
    severity text,
    paths text[] NOT NULL,
    text text NOT NULL,
    options jsonb,
    status text NOT NULL CHECK (status IN ('open', 'resolved', 'dismissed')),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    closed_at timestamp with time zone
);

CREATE INDEX sar_declaration_questions_declaration_id_idx ON sar_declaration_questions (declaration_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sar_declaration_questions;
DROP TABLE sar_generation_tasks;
DROP TABLE sar_generation_jobs;
-- +goose StatementEnd
