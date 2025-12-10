-- +goose Up
-- +goose StatementBegin

create table continuous_screening_dataset_updates (
    id uuid primary key default uuid_generate_v4(),
    dataset_name text not null,
    version text not null,
    delta_file_path text not null,
    total_items integer not null,
    created_at timestamp with time zone not null default current_timestamp,

    constraint unique_dataset_updates_name_version unique (dataset_name, version)
);

create table continuous_screening_update_jobs (
    id uuid primary key default uuid_generate_v4(),
    continuous_screening_dataset_update_id uuid not null,
    continuous_screening_config_id uuid not null,
    org_id uuid not null,
    status text not null CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    created_at timestamp with time zone not null default current_timestamp,
    updated_at timestamp with time zone not null default current_timestamp,

    constraint fk_continuous_screening_dataset_update foreign key (continuous_screening_dataset_update_id) references continuous_screening_dataset_updates (id) on delete cascade,
    constraint fk_continuous_screening_config foreign key (continuous_screening_config_id) references continuous_screening_configs (id) on delete cascade,
    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade
);

create table continuous_screening_job_offsets (
    id uuid primary key default uuid_generate_v4(),
    continuous_screening_update_job_id uuid not null,
    "offset" bigint not null,
    items_processed integer not null,
    created_at timestamp with time zone not null default current_timestamp,
    updated_at timestamp with time zone not null default current_timestamp,

    constraint fk_continuous_screening_update_job foreign key (continuous_screening_update_job_id) references continuous_screening_update_jobs (id) on delete cascade
    constraint unique_continuous_screening_job_offsets_update_job_id unique (continuous_screening_update_job_id)
);

create table continuous_screening_job_errors (
    id uuid primary key default uuid_generate_v4(),
    continuous_screening_update_job_id uuid not null,
    details jsonb,
    created_at timestamp with time zone not null default current_timestamp,

    constraint fk_continuous_screening_update_job_errors foreign key (continuous_screening_update_job_id) references continuous_screening_update_jobs (id) on delete cascade
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop table continuous_screening_job_errors;
drop table continuous_screening_job_offsets;
drop table continuous_screening_update_jobs;
drop table continuous_screening_dataset_updates;

-- +goose StatementEnd
