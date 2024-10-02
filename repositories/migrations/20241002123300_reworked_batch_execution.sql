-- +goose Up
-- +goose StatementBegin
CREATE TABLE
    decisions_to_create (
        id UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4 (),
        scheduled_execution_id UUID REFERENCES scheduled_executions (id) ON DELETE SET NULL,
        object_id VARCHAR(100) NOT NULL,
        status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (
            status IN ('pending', 'created', 'failed', 'trigger_condition_mismatch', 'retry')
        ),
        created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

CREATE INDEX decisions_to_create_scheduled_exec_id_idx ON decisions_to_create (scheduled_execution_id) INCLUDE (status);

CREATE INDEX decisions_to_create_unique_per_batch_idx ON decisions_to_create (scheduled_execution_id, object_id);

ALTER TABLE scheduled_executions
ADD COLUMN number_of_planned_decisions INT NOT NULL DEFAULT 0;

ALTER TABLE scheduled_executions
ADD COLUMN number_of_evaluated_decisions INT NOT NULL DEFAULT 0;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_to_create_unique_per_batch_idx;

DROP INDEX decisions_to_create_scheduled_exec_id_idx;

DROP TABLE decisions_to_create;

ALTER TABLE scheduled_executions
DROP COLUMN number_of_planned_decisions;

ALTER TABLE scheduled_executions
DROP COLUMN number_of_evaluated_decisions;

-- +goose StatementEnd