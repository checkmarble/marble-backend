-- +goose Up

create table scoring_settings (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,

    max_score integer not null,

    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now(),

    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade
);

create unique index idx_scoring_settings_org_id on scoring_settings (org_id);

create table scoring_scores (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,

    entity_type text not null,
    entity_id text not null,
    score int not null,
    source text not null,
    overriden_by uuid,

    created_at timestamp with time zone not null default now(),
    stale_at timestamp with time zone,
    deleted_at timestamp with time zone,

    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade
);

create index idx_scoring_scores
  on scoring_scores (org_id, entity_type, entity_id)
  include (score);

create unique index idx_scoring_active_scores
    on scoring_scores (org_id, entity_type, entity_id)
    include (score)
    where (deleted_at is null);

create table scoring_rulesets (
    id uuid primary key default gen_random_uuid(),
    org_id uuid not null,

    version integer not null default 1,
    name text not null,
    description text,
    entity_type text not null,
    thresholds int[] not null,
    cooldown_seconds integer not null default 0,

    constraint fk_org foreign key (org_id) references organizations (id) on delete cascade
);

create table scoring_rules (
    id uuid primary key default gen_random_uuid(),
    ruleset_id uuid not null,
    stable_id uuid not null,

    name text not null,
    description text,
    ast jsonb,

    constraint fk_ruleset foreign key (ruleset_id) references scoring_rulesets (id) on delete cascade
);

-- +goose Down

drop table scoring_rules;
drop table scoring_rulesets;
drop table scoring_settings;
drop table scoring_scores;
