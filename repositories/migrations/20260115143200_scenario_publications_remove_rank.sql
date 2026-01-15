-- +goose Up
-- +goose StatementBegin
ALTER TABLE scenario_publications
DROP COLUMN IF EXISTS rank;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE scenario_publications
ADD COLUMN rank SERIAL;

-- +goose StatementEnd