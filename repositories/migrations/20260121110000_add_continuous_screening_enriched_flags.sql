-- +goose Up
-- +goose StatementBegin

ALTER TABLE continuous_screenings ADD COLUMN opensanction_entity_enriched boolean NOT NULL DEFAULT false;
ALTER TABLE continuous_screening_matches ADD COLUMN enriched boolean NOT NULL DEFAULT false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE continuous_screening_matches DROP COLUMN enriched;
ALTER TABLE continuous_screenings DROP COLUMN opensanction_entity_enriched;

-- +goose StatementEnd
