-- +goose Up
-- +goose StatementBegin
CREATE TYPE audit_operation AS ENUM('INSERT', 'UPDATE', 'DELETE');

-- CreateTable
CREATE TABLE
	audit (
		"id" UUID NOT NULL DEFAULT gen_random_uuid () PRIMARY KEY,
		"operation" audit_operation NOT NULL,
		"user_id" TEXT,
		"table" VARCHAR NOT NULL,
		"entity_id" UUID NOT NULL,
		"data" JSONB NOT NULL DEFAULT '{}',
		"created_at" TIMESTAMPTZ (6) NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

-- Order audit trigger function
CREATE
OR REPLACE FUNCTION global_audit () RETURNS TRIGGER AS $$
    BEGIN
        IF (TG_OP = 'DELETE') THEN
            INSERT INTO audit ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('DELETE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, OLD.id, to_jsonb(OLD), now());

        ELSIF (TG_OP = 'UPDATE') THEN
            INSERT INTO audit ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('UPDATE', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());

        ELSIF (TG_OP = 'INSERT') THEN
            INSERT INTO audit ("operation", "user_id", "table", "entity_id", "data", "created_at")
            VALUES ('INSERT', current_setting('custom.current_user_id', TRUE), TG_TABLE_NAME, NEW.id, to_jsonb(NEW), now());
        END IF;
        RETURN NULL;
    END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit
AFTER INSERT
OR
UPDATE
OR DELETE ON custom_list_values FOR EACH ROW
EXECUTE FUNCTION global_audit ();

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TRIGGER audit ON custom_list_values;

DROP FUNCTION global_audit;

DROP TABLE audit;

DROP TYPE audit_operation;

-- +goose StatementEnd