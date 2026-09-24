-- +goose Up

create table customer_aggregates (
  id uuid primary key default gen_random_uuid(),
  org_id uuid not null references organizations (id) on delete cascade,
  table_id uuid not null references data_model_tables (id) on delete cascade,
  name text not null,
  kind text not null,
  expression jsonb not null,
  time_slice text not null
);

-- +goose Down

drop table customer_aggregates;
