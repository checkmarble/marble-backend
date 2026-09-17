-- +goose Up
-- +goose NO TRANSACTION

create index concurrently if not exists continuous_screening_update_jobs_stats_idx on continuous_screening_update_jobs(org_id, provider, continuous_screening_dataset_update_id);

-- +goose Down

drop index continuous_screening_update_jobs_stats_idx;
