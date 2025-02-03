-- +goose Up

create table sanction_check_files (
    id uuid primary key default uuid_generate_v4(),
    match_id uuid not null,
    bucket_name text not null,
    file_reference text not null,
    file_name text,
    created_at timestamp with time zone not null default now(),

    constraint fk_sanction_check_match
        foreign key (match_id)
        references sanction_check_matches (id)
);

-- +goose Down

drop table sanction_check_files;