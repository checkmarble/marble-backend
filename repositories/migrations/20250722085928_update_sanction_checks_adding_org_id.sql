-- +goose Up
-- +goose StatementBegin

alter table sanction_checks add column org_id uuid;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table sanction_checks drop column org_id;

-- +goose StatementEnd
