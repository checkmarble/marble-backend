-- +goose Up
-- +goose StatementBegin
ALTER TABLE sar_declaration_questions DROP CONSTRAINT sar_declaration_questions_status_check;
ALTER TABLE sar_declaration_questions ADD CONSTRAINT sar_declaration_questions_status_check
    CHECK (status IN ('open', 'resolved', 'dismissed', 'replaced'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE sar_declaration_questions SET status = 'dismissed' WHERE status = 'replaced';
ALTER TABLE sar_declaration_questions DROP CONSTRAINT sar_declaration_questions_status_check;
ALTER TABLE sar_declaration_questions ADD CONSTRAINT sar_declaration_questions_status_check
    CHECK (status IN ('open', 'resolved', 'dismissed'));
-- +goose StatementEnd
