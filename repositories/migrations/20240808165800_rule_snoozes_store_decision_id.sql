-- +goose Up
-- +goose StatementBegin
ALTER TABLE rule_snoozes
ADD COLUMN created_from_decision_id UUID REFERENCES decisions (id) ON DELETE SET NULL;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE rule_snoozes
DROP COLUMN created_from_decision_id;

-- +goose StatementBegin
-- +goose StatementEnd