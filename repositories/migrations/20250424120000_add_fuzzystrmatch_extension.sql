-- +goose Up

CREATE EXTENSION if not exists fuzzystrmatch SCHEMA public;

-- +goose Down
