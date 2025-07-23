-- +goose Up
-- +goose StatementBegin
ALTER TABLE sanction_checks ALTER COLUMN org_id SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sanction_checks ALTER COLUMN org_id DROP NOT NULL;
-- +goose StatementEnd
