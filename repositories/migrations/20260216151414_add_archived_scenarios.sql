-- +goose Up
alter table scenarios add column archived boolean not null default false;

-- +goose Down
alter table scenarios drop column archived;
