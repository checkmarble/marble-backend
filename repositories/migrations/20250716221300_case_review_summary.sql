-- +goose Up
-- +goose StatementBegin
create table
  ai_case_review_files (
    id uuid primary key default gen_random_uuid (),
    case_id uuid not null,
    status text not null,
    bucket_name text not null,
    file_reference text not null,
    reaction text,
    dto_version text not null,
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),
    constraint fk_case_id foreign key (case_id) references cases (id) on delete cascade,
    constraint status_check check (status in ('pending', 'completed', 'failed')),
    constraint reaction_check check (reaction in ('ok', 'ko'))
  );

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
drop table ai_case_review_files;

-- +goose StatementEnd