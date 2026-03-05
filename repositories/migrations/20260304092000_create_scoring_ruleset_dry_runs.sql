-- +goose Up

create table scoring_dry_runs (
    id uuid primary key default gen_random_uuid(),
    ruleset_id uuid not null,
    status text not null default 'pending',
    record_count int not null,
    results jsonb,
    created_at timestamp with time zone not null default now(),

    constraint fk_dry_run_rulesets foreign key (ruleset_id) references scoring_rulesets (id) on delete cascade
);

create unique index idx_dry_run_by_ruleset on scoring_dry_runs (ruleset_id) where status in ('pending', 'running');

-- +goose Down

drop table scoring_dry_runs;
