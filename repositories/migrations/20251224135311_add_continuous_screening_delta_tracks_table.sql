-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

create table continuous_screening_dataset_files (
    id uuid primary key default uuid_generate_v4 (),
    org_id uuid not null,
    file_type text not null,
    version text not null,
    file_path text not null,
    status text not null check (status in ('pending', 'processing', 'uploaded', 'failed')),
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),

    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade
);

create index idx_continuous_screening_dataset_files_org_type_status on continuous_screening_dataset_files (org_id, file_type, status);

create table continuous_screening_delta_tracks (
    id uuid primary key default uuid_generate_v4 (),
    org_id uuid not null,
    object_type text not null,
    object_id text not null,
    object_internal_id uuid not null,
    entity_id text not null,
    operation text not null,
    dataset_file_id uuid,
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),

    constraint fk_org foreign key (org_id) references organizations (id),
    constraint fk_dataset_file_id foreign key (dataset_file_id) references continuous_screening_dataset_files (id)
);

-- Get all unprocessed delta tracks grouped by org
create index idx_cs_delta_tracks_unprocessed_by_org on continuous_screening_delta_tracks (org_id) where dataset_file_id is null;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table continuous_screening_delta_tracks;
-- +goose StatementEnd
