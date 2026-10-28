-- +goose Up

-- Seed one watermark per counter metrics collector from the legacy global "metrics" watermark.
-- The list of names must match the CollectorName* constants in usecases/metrics_collection/constant.go,
-- which are permanent: renaming one would lose its watermark.
insert into watermarks (org_id, type, watermark_time)
select null, 'metrics:' || name, w.watermark_time
from watermarks w,
  unnest(array[
    'decisions',
    'cases',
    'screenings',
    'ai_case_reviews',
    'screenings_by_provider',
    'continuous_screenings_by_provider',
    'freeform_searches_by_provider'
  ]) as name
where w.org_id is null and w.type = 'metrics'
on conflict (org_id, type) do nothing;

-- +goose Down

-- The legacy watermark has kept advancing to "now" meanwhile, so the backlog of a lagging collector is not recovered.
delete from watermarks
where org_id is null and type in (
  'metrics:decisions',
  'metrics:cases',
  'metrics:screenings',
  'metrics:ai_case_reviews',
  'metrics:screenings_by_provider',
  'metrics:continuous_screenings_by_provider',
  'metrics:freeform_searches_by_provider'
);
