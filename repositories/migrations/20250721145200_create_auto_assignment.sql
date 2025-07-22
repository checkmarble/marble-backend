-- +goose Up

alter table organizations
    add column auto_assign_queue_limit int not null default 10;

alter table inboxes
    add column auto_assign_enabled bool not null default false;

alter table inbox_users
    add column auto_assignable bool not null default false;

create table user_unavailabilities (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,
    user_id uuid not null,
    from_date timestamp with time zone not null,
    until_date timestamp with time zone not null,
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),

    constraint fk_org_id foreign key (org_id) references organizations (id) on delete cascade,
    constraint fk_user_id foreign key (user_id) references users (id) on delete cascade
);

create index idx_user_avail_org_id_dates on user_unavailabilities (org_id, user_id, from_date, until_date);

-- +goose Down

alter table organizations drop column auto_assign_queue_limit;
alter table inboxes drop column auto_assign_enabled;
alter table inbox_users drop column auto_assignable;

drop table user_unavailabilities;
