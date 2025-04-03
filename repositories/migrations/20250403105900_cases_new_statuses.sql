-- +goose Up
-- +goose StatementBegin

-- TODO: let's determine if we want explicit concurrency control here (by locking the table, for example)

alter table cases
    add column outcome text not null default 'unset',
    add column waiting boolean not null default false,
    alter column status set default 'pending';

update cases
set
    outcome = 'unset',
    status =
        case status
            when 'open' then 'pending'
            when 'investigating' then 'investigating'
            else 'closed'
        end;

alter table cases
    add constraint cases_status_check check (status in ('pending', 'investigating', 'closed')),
    add constraint cases_outcome_check check (outcome in ('unset', 'false_positive', 'valuable_alert', 'confirmed_risk'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table cases drop constraint cases_status_check;

update cases
set
    status =
        case status
        when 'pending' then 'open'
        when 'closed' then 'resolved'
        else 'investigating'
        end;

alter table cases
    drop column outcome,
    drop column waiting,
    alter column status set default 'pending';

-- +goose StatementEnd