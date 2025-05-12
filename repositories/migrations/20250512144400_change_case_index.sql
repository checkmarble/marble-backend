-- +goose Up
-- +goose StatementBegin

alter index idx_inbox_cases rename to idx_inbox_cases_2;

create index idx_inbox_cases
    on cases (org_id, inbox_id, (boost is null), (assigned_to is not null), created_at desc, id desc);

drop index idx_inbox_cases_2;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter index idx_inbox_cases rename to idx_inbox_cases_2;

create index idx_inbox_cases
    on cases (org_id, inbox_id, (boost is null), created_at desc, id desc);

drop index idx_inbox_cases_2;

-- +goose StatementEnd