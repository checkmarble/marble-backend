-- +goose Up
-- +goose StatementBegin

create table roles (
  id uuid primary key default gen_random_uuid(),
  org_id uuid not null,
  slug text not null,
  name text not null,
  permissions text[] not null default '{}',

  unique (org_id, slug),
  unique (org_id, name)
);

alter table grants
  add column custom_role_id uuid references roles (id),
  add constraint grants_custom_role_check
    check ((custom_role_id is not null) = (role like 'org/%')),
  add constraint grants_custom_role_scope_check
    check (custom_role_id is null or organization_id is not null);

-- Recreated so that `select *` picks up the new column.
create or replace view active_grants as
select *
from grants
where
  revoked_at is null and
  (expires_at is null or expires_at > now());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- The view depends on the column to drop, so it is recreated afterwards.
drop view active_grants;

alter table grants
    drop constraint grants_custom_role_scope_check,
    drop constraint grants_custom_role_check,
    drop column custom_role_id;

create view active_grants as
select *
from grants
where revoked_at is null
  and (expires_at is null or expires_at > now());

drop table roles;

-- +goose StatementEnd
