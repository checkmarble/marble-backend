-- +goose Up

alter table screenings
    add column provider text not null default 'opensanctions';

-- +goose Down

alter table screenings
    drop column provider;
