-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS audit;

ALTER TABLE audit
SET SCHEMA audit;

ALTER TABLE audit.audit
RENAME TO audit_events;

CREATE
OR REPLACE FUNCTION global_audit () RETURNS TRIGGER AS $$
    BEGIN
        IF (TG_OP = 'DELETE') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('DELETE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());

        ELSIF (TG_OP = 'UPDATE') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('UPDATE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());

        ELSIF (TG_OP = 'INSERT') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('INSERT', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
        END IF;
        RETURN NULL;
    END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
CREATE
OR REPLACE FUNCTION global_audit () RETURNS TRIGGER AS $$
    BEGIN
        IF (TG_OP = 'DELETE') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('DELETE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());

        ELSIF (TG_OP = 'UPDATE') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('UPDATE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());

        ELSIF (TG_OP = 'INSERT') THEN
            INSERT INTO audit.audit_events ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('INSERT', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
        END IF;
        RETURN NULL;
    END;
$$ LANGUAGE plpgsql;

ALTER TABLE audit.audit_events
RENAME TO audit;

ALTER TABLE audit.audit
SET SCHEMA marble;

DROP SCHEMA audit CASCADE;

-- +goose StatementEnd