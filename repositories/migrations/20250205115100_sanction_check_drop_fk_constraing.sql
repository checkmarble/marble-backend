-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_checks
DROP CONSTRAINT fk_decision;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_checks
ADD CONSTRAINT fk_decision FOREIGN KEY (decision_id) REFERENCES decisions (id);

-- +goose StatementEnd