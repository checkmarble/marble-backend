-- +goose Up

alter table continuous_screening_dataset_files
    add column provider text not null default 'opensanctions';

-- +goose Down

alter table continuous_screening_dataset_files
    drop column provider;
