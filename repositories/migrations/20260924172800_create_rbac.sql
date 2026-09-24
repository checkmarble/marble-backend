-- +goose Up

create table roles (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,
    slug text not null,
    name text not null,
    permissions text[] not null default '{}',

    unique (org_id, slug),
    unique (org_id, name)
);

create table role_bindings (
    id uuid primary key default gen_random_uuid(),
    org_id uuid,
    user_id uuid references users (id) on delete cascade,
    api_key_id uuid references api_keys (id) on delete cascade,
    native_role text,
    custom_role_id uuid references roles (id) on delete cascade,
    conditions jsonb not null default '{}',

	constraint role_bindings_one_principal check (num_nonnulls(user_id, api_key_id) = 1),
	constraint role_bindings_one_role check (num_nonnulls(native_role, custom_role_id) = 1),
	constraint role_bindings_custom_role_has_org check (custom_role_id is null or org_id is not null)
);

create index idx_role_bindings_user on role_bindings (user_id) where user_id is not null;
create index idx_role_bindings_api_key on role_bindings (api_key_id) where api_key_id is not null;

insert into role_bindings (org_id, user_id, native_role)
select organization_id, id, case role
    when 1 then 'VIEWER'
    when 2 then 'BUILDER'
    when 3 then 'PUBLISHER'
    when 4 then 'ADMIN'
    when 5 then 'API_CLIENT'
    when 6 then 'MARBLE_ADMIN'
    when 9 then 'ANALYST'
end
from users
where role in (1, 2, 3, 4, 5, 6, 9);

insert into role_bindings (org_id, api_key_id, native_role)
select org_id, id, case role
    when 1 then 'VIEWER'
    when 2 then 'BUILDER'
    when 3 then 'PUBLISHER'
    when 4 then 'ADMIN'
    when 5 then 'API_CLIENT'
    when 6 then 'MARBLE_ADMIN'
    when 9 then 'ANALYST'
end
from api_keys
where role in (1, 2, 3, 4, 5, 6, 9);

-- +goose Down

drop table role_bindings;
drop table roles;
