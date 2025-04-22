-- +goose Up

create table data_model_options (
  id uuid default gen_random_uuid(),
  table_id uuid not null unique,
  displayed_fields uuid[] not null default '{}',
  field_order uuid[] not null default '{}',

  primary key (id),
  constraint fk_data_model_table
    foreign key (table_id) references data_model_tables (id)
    on delete cascade
);

-- +goose Down

drop table data_model_options;
