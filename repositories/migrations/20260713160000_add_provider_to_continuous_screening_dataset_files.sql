-- +goose Up

alter table continuous_screening_dataset_files
    add column provider text not null default '';

update continuous_screening_dataset_files dataset_file
set provider = update_job.provider
from (
    select distinct on (org_id)
        org_id,
        provider
    from continuous_screening_update_jobs
    order by org_id, created_at desc
) update_job
where dataset_file.org_id = update_job.org_id;

-- +goose Down

alter table continuous_screening_dataset_files
    drop column provider;
