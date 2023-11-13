-- +goose Up
-- +goose StatementBegin
ALTER TABLE decisions
ADD COLUMN case_id UUID;
CREATE INDEX decisions_case_id_idx ON decisions(org_id, case_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX decisions_case_id_idx;
ALTER TABLE decisions DROP COLUMN case_id;
-- +goose StatementEnd
