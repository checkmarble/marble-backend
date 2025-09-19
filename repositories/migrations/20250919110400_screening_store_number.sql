-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_checks
ADD COLUMN number_of_matches INT;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_checks
DROP COLUMN number_of_matches;

-- +goose StatementEnd