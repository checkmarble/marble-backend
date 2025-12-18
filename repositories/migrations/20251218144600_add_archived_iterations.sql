-- +goose Up

alter table scenario_iterations
    add column archived boolean default false;


alter table data_model_fields
    add column archived boolean default false;

alter table data_model_fields
    drop constraint data_model_fields_table_id_name_key;

alter table data_model_fields
    drop constraint unique_data_model_fields_name;

create unique index data_model_fields_table_id_name
    on data_model_fields (table_id, name) where (archived is false);

-- +goose Down

alter table scenario_iterations
    drop column archived;

alter table data_model_fields
    drop column archived;

drop index data_model_fields_table_id_name;

create unique index data_model_fields_table_id_name_key
	on data_model_fields (table_id, name);

alter table data_model_fields
    add constraint unique_data_model_fields_name
    unique (table_id, name);
