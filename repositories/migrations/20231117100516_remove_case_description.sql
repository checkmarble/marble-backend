-- +goose Up
-- +goose StatementBegin
ALTER TABLE cases DROP COLUMN description;
ALTER TABLE case_events ALTER COLUMN new_value TYPE text;
ALTER TABLE case_events ALTER COLUMN previous_value TYPE text;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE cases ADD COLUMN description text;
ALTER TABLE case_events ALTER COLUMN new_value TYPE varchar;
ALTER TABLE case_events ALTER COLUMN previous_value TYPE varchar;
-- +goose StatementEnd
