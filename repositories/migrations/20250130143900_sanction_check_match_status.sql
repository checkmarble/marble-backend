-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_check_matches
DROP CONSTRAINT IF EXISTS sanction_check_matches_status_check,
ADD CONSTRAINT sanction_check_matches_status_check CHECK (status IN ('pending', 'confirmed_hit', 'no_hit', 'skipped'));

ALTER TABLE sanction_checks
DROP CONSTRAINT IF EXISTS sanction_checks_status_check,
ADD constraint sanction_checks_status_check CHECK (status IN ('confirmed_hit', 'no_hit', 'in_review', 'error', 'too_many_hits'));

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_check_matches
DROP CONSTRAINT IF EXISTS sanction_check_matches_status_check,
ADD CONSTRAINT sanction_check_matches_status_check CHECK (status IN ('pending', 'confirmed_hit', 'no_hit'));

ALTER TABLE sanction_checks
DROP CONSTRAINT IF EXISTS sanction_checks_status_check,
ADD constraint sanction_checks_status_check CHECK (status IN ('confirmed_hit', 'no_hit', 'in_review', 'error'));

-- +goose StatementEnd