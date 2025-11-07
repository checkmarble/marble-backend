-- +goose Up
-- +goose StatementBegin

create table continuous_screening_configs (
    id uuid primary key default uuid_generate_v4(),
    org_id uuid not null,
    name text not null,
    description text,
    datasets text[] not null,
    match_threshold int not null check (match_threshold between 0 and 100),
    match_limit int not null,
    created_at timestamp with time zone not null default current_timestamp,
    updated_at timestamp with time zone not null default current_timestamp,
    enabled boolean not null default true,

    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade
);

create index idx_continuous_screening_configs_org_id on continuous_screening_configs (org_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop table continuous_screening_configs;

-- +goose StatementEnd
