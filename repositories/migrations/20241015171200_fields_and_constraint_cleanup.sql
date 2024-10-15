-- +goose Up
-- +goose StatementBegin
-- deprecated fields cleanup
ALTER TABLE decisions
DROP COLUMN error_code;

ALTER TABLE decision_rules
DROP COLUMN name;

ALTER TABLE decision_rules
DROP COLUMN description;

ALTER TABLE decision_rules
DROP COLUMN deleted_at;

-- unnecessary "max len 10" in varchar
ALTER TABLE decision_rules
ALTER COLUMN outcome
TYPE VARCHAR;

-- on a big table like decision rules, those constraints are having quite an impact on write performance, for not too much added value.
-- We'll need to cleanup manually orphan decision_rules when we remove organizations though (but that's ok)
ALTER TABLE decision_rules
DROP CONSTRAINT fk_decision_rules_org;

ALTER TABLE decision_rules
DROP CONSTRAINT fk_decision_rules_decisions;

ALTER TABLE decision_rules
DROP CONSTRAINT decision_rules_rule_id_fkey;

-- with the constraints above gone, we also don't need the indexes that were just used for better org delete speed
DROP INDEX decision_rules_rule_id_idx;

DROP INDEX decision_rules_org_id_idx;

-- another unused field
ALTER TABLE decisions
DROP COLUMN deleted_at;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE decisions
ADD COLUMN error_code INTEGER;

ALTER TABLE decision_rules
ADD COLUMN name VARCHAR;

ALTER TABLE decision_rules
ADD COLUMN description VARCHAR;

ALTER TABLE decision_rules
ADD COLUMN deleted_at TIMESTAMP;

ALTER TABLE decision_rules
ALTER COLUMN outcome
TYPE VARCHAR(10);

ALTER TABLE decision_rules
ADD CONSTRAINT fk_decision_rules_org FOREIGN KEY (org_id) REFERENCES organizations (id) ON DELETE CASCADE;

ALTER TABLE decision_rules
ADD CONSTRAINT fk_decision_rules_decisions FOREIGN KEY (decision_id) REFERENCES decisions (id) ON DELETE CASCADE;

ALTER TABLE decision_rules
ADD CONSTRAINT decision_rules_rule_id_fkey FOREIGN KEY (rule_id) REFERENCES decision_rules (id) ON DELETE CASCADE;

CREATE INDEX decision_rules_rule_id_idx ON decision_rules (rule_id);

CREATE INDEX decision_rules_org_id_idx ON decision_rules (org_id);

ALTER TABLE decisions
ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;

-- +goose StatementEnd