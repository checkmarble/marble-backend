-- +goose Up
-- +goose StatementBegin

ALTER TABLE continuous_screenings ADD COLUMN opensanction_entity_id text;
ALTER TABLE continuous_screenings ADD COLUMN opensanction_entity_payload jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE continuous_screenings DROP COLUMN opensanction_entity_id;
ALTER TABLE continuous_screenings DROP COLUMN opensanction_entity_payload;

-- +goose StatementEnd
