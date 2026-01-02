-- +goose Up

create table data_model_field_aliases (
    id uuid primary key default gen_random_uuid(),
    table_id uuid not null,
    field_id uuid not null,
    name text,
    created_at timestamp with time zone not null default now(),

    unique (table_id, name),

    constraint fk_table foreign key (table_id) references data_model_tables (id) on delete cascade,
    constraint fk_field foreign key (field_id) references data_model_fields (id) on delete cascade
);

create index idx_data_model_aliases_field_id on data_model_field_aliases (field_id);

-- +goose Down

drop table data_model_field_aliases;
