-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_check_matches
DROP CONSTRAINT IF EXISTS sanction_check_matches_status_check,
add constraint sanction_check_matches_status_check check (status in ('pending', 'confirmed_hit', 'no_hit', 'skipped'));

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_check_matches
DROP CONSTRAINT IF EXISTS sanction_check_matches_status_check,
add constraint sanction_check_matches_status_check check (status in ('pending', 'confirmed_hit', 'no_hit'));

-- +goose StatementEnd