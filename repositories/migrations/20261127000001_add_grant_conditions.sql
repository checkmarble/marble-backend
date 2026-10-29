-- +goose Up
-- +goose StatementBegin

alter table grants
    add column conditions jsonb not null default '{}';

-- Recreated so that `select *` picks up the new column.
create or replace view active_grants as
select *
from grants
where revoked_at is null
  and (expires_at is null or expires_at > now());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop view active_grants;

alter table grants
    drop column conditions;

create view active_grants as
select *
from grants
where revoked_at is null
  and (expires_at is null or expires_at > now());

-- +goose StatementEnd
