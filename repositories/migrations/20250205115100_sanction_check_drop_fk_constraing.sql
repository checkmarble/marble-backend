-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_checks
DROP CONSTRAINT fk_decision;

ALTER TABLE sanction_checks
ADD COLUMN initial_has_matches bool NOT NULL DEFAULT false,
ADD COLUMN match_limit int NOT NULL DEFAULT 0,
ALTER COLUMN search_threshold
SET NOT NULL DEFAULT 0;

ALTER TABLE organizations
ALTER COLUMN sanctions_threshold NOT NULL DEFAULT 70,
ALTER COLUMN sanctions_limit NOT NULL DEFAULT 30;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_checks
ADD CONSTRAINT fk_decision FOREIGN KEY (decision_id) REFERENCES decisions (id);

ALTER TABLE sanction_checks
DROP COLUMN initial_has_matches,
DROP COLUMN match_limit,
ALTER COLUMN search_threshold
DROP NOT NULL,
ALTER COLUMN search_threshold
DROP DEFAULT;

ALTER TABLE organizations
ALTER COLUMN sanctions_threshold
DROP NOT NULL,
ALTER COLUMN sanctions_threshold
DROP DEFAULT,
ALTER COLUMN sanctions_limit
DROP NOT NULL,
ALTER COLUMN sanctions_limit
DROP DEFAULT;

-- +goose StatementEnd