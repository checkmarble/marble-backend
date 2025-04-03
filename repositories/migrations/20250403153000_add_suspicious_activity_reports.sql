-- +goose Up

create table suspicious_activity_reports (
    id uuid primary key default gen_random_uuid(),
    report_id uuid not null default gen_random_uuid(),
    case_id uuid not null,
    status text not null check (status in ('pending', 'completed')),
    bucket text,
    blob_key text,
    created_by uuid not null,
    uploaded_by uuid,
    created_at timestamp with time zone not null default now(),
    deleted_at timestamp with time zone,

    constraint fk_case_id foreign key (case_id) references cases (id) on delete cascade,
    constraint fk_created_by foreign key (created_by) references users (id),
    constraint fk_uploaded_by foreign key (uploaded_by) references users (id)
);

create unique index idx_live_suspicious_activity_reports on suspicious_activity_reports (case_id, report_id) where (deleted_at is null);

-- +goose Down

drop table suspicious_activity_reports;