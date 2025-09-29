-- +goose Up

alter table analytics_settings
    alter column id set default gen_random_uuid(),
    alter column created_at set default now(),
    alter column updated_at set default now(),
    add constraint fk_org foreign key (org_id) references organizations (id),
    add constraint unq_org_id_table unique (org_id, trigger_object_type);

-- +goose Down

alter table analytics_settings
    drop constraint fk_org,
    drop constraint unq_org_id_table;
