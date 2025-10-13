-- +goose Up

alter table users
    add column picture text default '';

-- +goose Down

alter table users
    drop column picture;
