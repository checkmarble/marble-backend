-- +goose Up

alter table data_model_tables
    add column alias text not null default '',
    add column semantic_type text not null default '',
    add column caption_field text not null default '';

-- +goose Down

alter table data_model_tables
  drop column alias,
  drop column semantic_type,
  drop column caption_field;
