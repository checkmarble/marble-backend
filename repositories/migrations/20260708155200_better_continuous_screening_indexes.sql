-- +goose Up
-- +goose NO TRANSACTION

create index concurrently idx_continuous_screenings_case_id
    on continuous_screenings (org_id, case_id)
    where case_id is not null;

drop index idx_continuous_screenings_object_id;
drop index idx_continuous_screenings_org_id;

-- +goose Down
-- +goose NO TRANSACTION

create index concurrently idx_continuous_screenings_object_id
	on continuous_screenings (object_id);

create index concurrently idx_continuous_screenings_org_id
	on continuous_screenings (org_id);

drop index idx_continuous_screenings_case_id;
