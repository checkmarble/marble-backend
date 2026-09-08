-- +goose Up
-- +goose NO TRANSACTION

create unique index concurrently if not exists enum_values_coalesced on
  data_model_enum_values(field_id, coalesce(text_value, float_value::text))
  include (text_value, float_value);

-- +goose Down

drop index concurrently if exists enum_values_coalesced;
