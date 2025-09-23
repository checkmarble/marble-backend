-- +goose Up

create table analytics_settings (
    id uuid primary key,
    org_id uuid not null,
    trigger_object_type text not null,
    trigger_fields text[],
    db_fields jsonb,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

create index idx_org_trigger_object on analytics_settings (org_id, trigger_object_type);

alter table decisions
    add column analytics_fields jsonb default null;

-- +goose Down

drop table analytics_settings;

alter table decisions
    drop column analytics_fields;
