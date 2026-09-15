-- +goose Up

create table screening_saved_searches (
  id uuid primary key default gen_random_uuid(),
  org_id uuid not null references organizations (id),
  name text not null,
  provider text not null default 'opensanctions',
  config jsonb not null,
  created_at timestamptz not null default now(),
  deleted_at timestamptz
);

create unique index unq_screening_saved_search_named
  on screening_saved_searches (org_id, name)
  where deleted_at is null;

create index idx_screening_saved_search_org
  on screening_saved_searches (org_id, provider)
  where deleted_at is null;

-- +goose Down

drop table screening_saved_searches;
