-- +goose Up
-- +goose StatementBegin
create table
    sanction_checks (
        id uuid default uuid_generate_v4 (),
        decision_id uuid not null,
        status text not null check (status in ('confirmed_hit', 'no_hit', 'in_review', 'error', 'too_many_hits')) default 'in_review',
        search_input jsonb,
        search_datasets text[],
        match_threshold integer not null,
        match_limit int not null,
        is_manual bool default false,
        is_partial bool default false,
        is_archived bool default false,
        initial_has_matches bool NOT NULL DEFAULT false,
        requested_by uuid null,
        created_at timestamp with time zone default now(),
        updated_at timestamp with time zone default now(),
        primary key (id),
        constraint fk_user foreign key (requested_by) references users (id)
    );

create index idx_sanction_checks_decision_id on sanction_checks (decision_id);

create table
    sanction_check_matches (
        id uuid default uuid_generate_v4 (),
        sanction_check_id uuid not null,
        opensanction_entity_id text not null,
        status text check (status in ('pending', 'confirmed_hit', 'no_hit', 'skipped')) default 'pending',
        query_ids text[],
        payload jsonb,
        reviewed_by uuid,
        created_at timestamp with time zone default now(),
        updated_at timestamp with time zone default now(),
        primary key (id),
        constraint fk_sanction_check foreign key (sanction_check_id) references sanction_checks (id),
        constraint fk_user foreign key (reviewed_by) references users (id)
    );

create index idx_sanction_check_matches_sanction_check_id on sanction_check_matches (sanction_check_id);

create table
    sanction_check_match_comments (
        id uuid default uuid_generate_v4 (),
        sanction_check_match_id uuid not null,
        commented_by uuid not null,
        comment text not null default '',
        created_at timestamp with time zone default now(),
        primary key (id),
        constraint fk_sanction_check_match foreign key (sanction_check_match_id) references sanction_check_matches (id),
        constraint fk_user foreign key (commented_by) references users (id)
    );

create index idx_sanction_check_match_comments_sanction_check_match_id on sanction_check_match_comments (sanction_check_match_id);

create table
    sanction_check_files (
        id uuid primary key default uuid_generate_v4 (),
        sanction_check_id uuid not null,
        bucket_name text not null,
        file_reference text not null,
        file_name text,
        created_at timestamp with time zone not null default now(),
        constraint fk_sanction_check_match foreign key (sanction_check_id) references sanction_checks (id)
    );

create index idx_sanction_check_files_sanction_check_id on sanction_check_files (sanction_check_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
drop table sanction_check_match_comments;

drop table sanction_check_matches;

drop table sanction_check_files;

drop table sanction_checks;

-- +goose StatementEnd