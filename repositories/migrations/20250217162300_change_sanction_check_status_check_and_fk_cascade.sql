-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_check_matches
DROP CONSTRAINT fk_sanction_check,
ADD CONSTRAINT fk_sanction_check FOREIGN KEY (sanction_check_id) REFERENCES sanction_checks (id) ON DELETE CASCADE;

ALTER TABLE sanction_check_match_comments
DROP CONSTRAINT fk_sanction_check_match,
ADD CONSTRAINT fk_sanction_check_match FOREIGN KEY (sanction_check_match_id) REFERENCES sanction_check_matches (id) ON DELETE CASCADE;

ALTER TABLE sanction_check_files
DROP CONSTRAINT fk_sanction_check_match,
ADD CONSTRAINT fk_sanction_check_match FOREIGN KEY (sanction_check_id) REFERENCES sanction_checks (id) ON DELETE CASCADE;

ALTER TABLE sanction_checks
DROP CONSTRAINT IF EXISTS sanction_checks_status_check;

delete from sanction_checks
where
    status = 'too_many_hits';

-- Then add the new constraint with updated values
ALTER TABLE sanction_checks
ADD CONSTRAINT sanction_checks_status_check CHECK (status in ('confirmed_hit', 'no_hit', 'in_review', 'error'));

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_checks
DROP CONSTRAINT IF EXISTS sanction_checks_status_check;

ALTER TABLE sanction_checks
ADD CONSTRAINT sanction_checks_status_check CHECK (status in ('confirmed_hit', 'no_hit', 'in_review', 'error', 'too_many_hits'));

ALTER TABLE sanction_check_matches
DROP CONSTRAINT fk_sanction_check,
ADD CONSTRAINT fk_sanction_check FOREIGN KEY (sanction_check_id) REFERENCES sanction_checks (id);

ALTER TABLE sanction_check_match_comments
DROP CONSTRAINT fk_sanction_check_match,
ADD CONSTRAINT fk_sanction_check_match FOREIGN KEY (sanction_check_match_id) REFERENCES sanction_check_matches (id);

ALTER TABLE sanction_check_files
DROP CONSTRAINT fk_sanction_check_match,
ADD CONSTRAINT fk_sanction_check_match FOREIGN KEY (sanction_check_id) REFERENCES sanction_checks (id);

-- +goose StatementEnd