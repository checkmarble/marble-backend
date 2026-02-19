-- +goose Up
ALTER TABLE screenings ADD COLUMN counterparty_id text;

-- +goose Down
ALTER TABLE screenings DROP COLUMN counterparty_id;
