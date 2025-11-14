-- +goose Up
-- +goose StatementBegin

alter table continuous_screening_configs
    alter column stable_id type text;
create index idx_continuous_screening_configs_stable_id_created_at_desc
    on continuous_screening_configs (stable_id, created_at desc);

alter table continuous_screening
    add column continuous_screening_config_stable_id text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

alter table continuous_screening_configs
    alter column stable_id type uuid using stable_id::uuid;
drop index idx_continuous_screening_configs_stable_id_created_at_desc;

alter table continuous_screening
    drop column continuous_screening_config_stable_id;

-- +goose StatementEnd
