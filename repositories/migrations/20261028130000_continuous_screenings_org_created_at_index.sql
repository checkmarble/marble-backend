-- +goose Up
-- +goose NO TRANSACTION

-- Serves the metrics count of continuous screenings by provider (CountCSScreeningsByProvider). The predicate must stay
-- identical to that query's filter, otherwise the planner cannot use this partial index.
create index concurrently if not exists idx_continuous_screenings_org_created_at
    on continuous_screenings (org_id, created_at)
    include (provider)
    where trigger_type in ('object_added', 'object_updated') and status <> 'error';

-- +goose Down
-- +goose NO TRANSACTION

drop index concurrently if exists idx_continuous_screenings_org_created_at;
