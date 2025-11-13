-- +goose Up
-- +goose StatementBegin
create table continuous_screening (
    id uuid primary key default uuid_generate_v4 (),
    org_id uuid not null,
    continuous_screening_config_id uuid not null,
    object_type text not null,
    object_id text not null,
    object_internal_id uuid not null,
    status text not null check (status in ('in_review', 'confirmed_hit', 'no_hit', 'skipped', 'error', 'too_many_hits')) default 'in_review',
    search_input jsonb,
    is_partial boolean default false,
    number_of_matches integer default 0,
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),

    constraint fk_continuous_screening_config foreign key (continuous_screening_config_id) references continuous_screening_configs (id),
    constraint fk_org foreign key (org_id) references organizations (id)
);

create index idx_continuous_screening_org_id on continuous_screening (org_id);
create index idx_continuous_screening_object_id on continuous_screening (object_id);

create table continuous_screening_matches (
    id uuid primary key default uuid_generate_v4 (),
    continuous_screening_id uuid not null,
    opensanction_entity_id text not null,
    status text check (status in ('pending', 'confirmed_hit', 'no_hit', 'skipped')) default 'pending',
    payload jsonb,
    reviewed_by uuid,
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),

    constraint fk_continuous_screening foreign key (continuous_screening_id) references continuous_screening (id),
    constraint fk_reviewed_by_user foreign key (reviewed_by) references users (id)
);

alter table continuous_screening_matches set (toast_tuple_target = 512);

create index idx_continuous_screening_matches_continuous_screening_id on continuous_screening_matches (continuous_screening_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table continuous_screening_matches;
drop table continuous_screening;
-- +goose StatementEnd
