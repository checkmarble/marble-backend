-- +goose Up

alter table scenario_iterations
    add column archived boolean default false;

alter table data_model_fields
    add column archived boolean default false;

alter table data_model_fields
    drop constraint if exists data_model_fields_table_id_name_key;

drop index if exists data_model_fields_table_id_name_key;

alter table data_model_fields
    drop constraint unique_data_model_fields_name;

create unique index data_model_fields_table_id_name
    on data_model_fields (table_id, name) where (archived is false);

-- +goose Down

delete from data_model_fields
where archived is true;

alter table scenario_iterations
    drop column archived;

alter table data_model_fields
    drop column archived;

alter table data_model_fields
    add constraint unique_data_model_fields_name
    unique (table_id, name);
