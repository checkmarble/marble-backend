-- +goose Up

alter table screening_configs
    add column weights jsonb;

alter table continuous_screening_configs
    add column weights jsonb;

-- +goose Down

alter table screening_configs
    drop column weights;

alter table continuous_screening_configs
    drop column weights;
