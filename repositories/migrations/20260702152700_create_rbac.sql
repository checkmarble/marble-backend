-- +goose Up

create table roles (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,
    name text not null,

    unique (org_id, name)
);

create table permissions (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,
    role_id uuid not null references roles (id),
    name text not null,
    condition text,

    unique (org_id, role_id, name)
);

alter table users
    add column roles text[] not null default '{}';

alter table api_keys
    add column roles text[] not null default '{}';

alter table users
    alter column role drop not null;

update users
    set roles = (case
        when role = 0 then '{}'
        when role = 1 then array['VIEWER']
        when role = 2 then array['BUILDER']
        when role = 3 then array['PUBLISHER']
        when role = 4 then array['ADMIN']
        when role = 5 then array['API_CLIENT']
        when role = 6 then array['MARBLE_ADMIN']
        when role = 9 then array['ANALYST']
        else '{}'
    end);

update api_keys
    set roles = (case
        when role = 0 then '{}'
        when role = 1 then array['VIEWER']
        when role = 2 then array['BUILDER']
        when role = 3 then array['PUBLISHER']
        when role = 4 then array['ADMIN']
        when role = 5 then array['API_CLIENT']
        when role = 6 then array['MARBLE_ADMIN']
        when role = 9 then array['ANALYST']
        else '{}'
    end);

-- +goose Down

alter table users
    drop column roles;

alter table api_keys
    drop column roles;

alter table users
    alter column role set not null;

drop table roles;
