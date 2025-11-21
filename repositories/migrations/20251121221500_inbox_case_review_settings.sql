-- +goose Up
-- +goose StatementBegin
ALTER TABLE inboxes
ADD COLUMN case_review_manual BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN case_review_on_case_created BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN case_review_on_escalate BOOLEAN NOT NULL DEFAULT false;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE inboxes
DROP COLUMN case_review_manual,
DROP COLUMN case_review_on_case_created,
DROP COLUMN case_review_on_escalate;

-- +goose StatementEnd