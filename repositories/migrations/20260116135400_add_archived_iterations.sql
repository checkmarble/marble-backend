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

create or replace trigger scenario_iterations_archived
after update of archived
on scenario_iterations
for each row execute function global_audit();

create or replace trigger data_model_tables
after delete
on data_model_tables
for each row execute function global_audit();

create or replace trigger data_model_fields
after update of archived or delete
on data_model_fields
for each row execute function global_audit();

create or replace trigger data_model_links
after delete
on data_model_links
for each row execute function global_audit();

create or replace trigger data_model_pivots
after delete
on data_model_pivots
for each row execute function global_audit();

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

drop trigger if exists scenario_iterations_archived on scenario_iterations;
drop trigger if exists data_model_tables on data_model_tables;
drop trigger if exists data_model_fields on data_model_fields;
drop trigger if exists data_model_links on data_model_links;
drop trigger if exists data_model_pivots on data_model_pivots;
