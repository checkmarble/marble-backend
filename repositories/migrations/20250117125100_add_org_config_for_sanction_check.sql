-- +goose Up
-- +goose StatementBegin

alter table organizations add column sanctioncheck_datasets text[];
alter table organizations add column sanctioncheck_threshold int check (sanctioncheck_threshold between 0 and 100);
alter table organizations add column sanctioncheck_limit int;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table organizations drop column sanctioncheck_datasets;
alter table organizations drop column sanctioncheck_threshold;
alter table organizations drop column sanctioncheck_limit;

-- +goose StatementEnd
