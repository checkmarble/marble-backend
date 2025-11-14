-- +goose Up
-- +goose StatementBegin

alter table continuous_screening_configs
    alter column stable_id type text;
create index idx_continuous_screening_configs_stable_id_created_at_desc
    on continuous_screening_configs (stable_id, created_at desc);
create unique index idx_continuous_screening_configs_stable_id_org_id_enabled
    on continuous_screening_configs (stable_id, org_id) 
    where enabled = true;

alter table continuous_screening
    add column continuous_screening_config_stable_id text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop index idx_continuous_screening_configs_stable_id_org_id_enabled;
drop index idx_continuous_screening_configs_stable_id_created_at_desc;
alter table continuous_screening_configs
    alter column stable_id type uuid using stable_id::uuid;

alter table continuous_screening
    drop column continuous_screening_config_stable_id;

-- +goose StatementEnd
