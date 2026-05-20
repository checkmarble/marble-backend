-- +goose Up

alter table continuous_screening_configs
    add column provider text not null default 'opensanctions',
    add column filters jsonb not null default '{}';

alter table continuous_screening_update_jobs
    add column provider text not null default 'opensanctions';

alter table continuous_screenings
    add column provider text not null default 'opensanctions';

-- +goose Down

alter table continuous_screening_configs
    drop column provider,
    drop column filters;
