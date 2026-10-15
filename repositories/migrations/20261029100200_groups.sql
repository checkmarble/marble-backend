-- +goose Up
-- +goose StatementBegin

create table groups (
    id uuid primary key,
    organization_id uuid not null references organizations(id) on delete cascade,
    name text not null check (length(trim(name)) > 0),
    created_at timestamptz not null default now(),
    deleted_at timestamptz
);
create unique index groups_org_name_unique on groups (organization_id, lower(name)) where deleted_at is null;

create table group_memberships (
    id uuid not null default gen_random_uuid() unique,
    group_id uuid not null references groups(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (group_id, user_id)
);
create index group_memberships_user_idx on group_memberships (user_id);

alter table grants drop constraint grants_principal_type_check;
alter table grants add constraint grants_principal_type_check
    check (principal_type in ('user', 'api_key', 'group'));
alter table grants add constraint grants_group_scope_check
    check (principal_type <> 'group' or (organization_id is not null and tenant_id is null));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

delete from grants where principal_type = 'group';
alter table grants drop constraint grants_group_scope_check;
alter table grants drop constraint grants_principal_type_check;
alter table grants add constraint grants_principal_type_check
    check (principal_type in ('user', 'api_key'));
drop table group_memberships;
drop table groups;

-- +goose StatementEnd
