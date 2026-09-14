-- +goose Up

alter table data_model_tables
  add column lifecycle jsonb;

-- +goose Down

alter table data_model_tables
  drop column lifecycle;
