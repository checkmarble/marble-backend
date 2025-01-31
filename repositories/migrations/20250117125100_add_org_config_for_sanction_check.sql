-- +goose Up
-- +goose StatementBegin

alter table organizations add column sanctions_threshold int check (sanctions_threshold between 0 and 100);
alter table organizations add column sanctions_limit int;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table organizations drop column sanctions_threshold;
alter table organizations drop column sanctions_limit;

-- +goose StatementEnd
