-- +goose Up
-- +goose StatementBegin

lock table cases in share mode;

alter table cases
    add column outcome text not null default 'unset',
    alter column status set default 'pending';

-- Backup of the tuples (id, status) into a temporary table so we
-- can rollback the migration if needs be.

create table tmp_case_statuses as
select id, status
from cases;

alter table tmp_case_statuses add primary key (id);

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

lock table cases in share mode;

-- We copy the backed up statuses from the temporary table, falling
-- back to a hardcoded mapping for cases that were created since then.

update cases
set
    status = coalesce(
        b.status,
        case cases.status
        when 'pending' then 'open'
        when 'closed' then 'resolved'
        else 'investigating'
        end
    )
from cases c
left join tmp_case_statuses b on c.id = b.id
where cases.id = c.id;

drop table tmp_case_statuses;

alter table cases
    drop column outcome,
    alter column status set default 'pending';

-- +goose StatementEnd