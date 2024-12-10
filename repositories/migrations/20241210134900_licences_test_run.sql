-- +goose Up
-- +goose StatementBegin
ALTER TABLE licenses
ADD COLUMN test_run BOOL NOT NULL DEFAULT FALSE;

-- +goose StatementEnd
-- +goose Down
ALTER TABLE licenses
DROP COLUMN test_run;

-- +goose StatementBegin
-- +goose StatementEnd