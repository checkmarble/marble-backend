-- +goose Up
-- +goose StatementBegin

alter table continuous_screening_configs
    alter column stable_id type text;
create index idx_continuous_screening_configs_stable_id_created_at_desc
    on continuous_screening_configs (stable_id, created_at desc);
create unique index idx_continuous_screening_configs_stable_id_org_id_enabled
    on continuous_screening_configs (stable_id, org_id)
    where enabled = true;

alter table continuous_screening rename to continuous_screenings;
alter index idx_continuous_screening_org_id rename to idx_continuous_screenings_org_id;
alter index idx_continuous_screening_object_id rename to idx_continuous_screenings_object_id;
alter table continuous_screenings
    add column continuous_screening_config_stable_id text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop index idx_continuous_screening_configs_stable_id_org_id_enabled;
drop index idx_continuous_screening_configs_stable_id_created_at_desc;
alter table continuous_screening_configs
    alter column stable_id type uuid using stable_id::uuid;

alter table continuous_screenings
    drop column continuous_screening_config_stable_id;
alter index idx_continuous_screenings_org_id rename to idx_continuous_screening_org_id;
alter index idx_continuous_screenings_object_id rename to idx_continuous_screening_object_id;
alter table continuous_screenings rename to continuous_screening;

-- +goose StatementEnd
