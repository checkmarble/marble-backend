-- +goose Up
-- +goose StatementBegin

ALTER TABLE continuous_screenings ADD COLUMN opensanction_entity_id text;
ALTER TABLE continuous_screenings ADD COLUMN opensanction_entity_payload jsonb;

ALTER TABLE continuous_screenings ALTER COLUMN object_type DROP NOT NULL;
ALTER TABLE continuous_screenings ALTER COLUMN object_id DROP NOT NULL;
ALTER TABLE continuous_screenings ALTER COLUMN object_internal_id DROP NOT NULL;

ALTER TABLE continuous_screenings ADD CONSTRAINT continuous_screenings_object_or_opensanction_check
CHECK (
    (object_type IS NOT NULL AND object_id IS NOT NULL AND object_internal_id IS NOT NULL)
    OR
    (opensanction_entity_id IS NOT NULL AND opensanction_entity_payload IS NOT NULL)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE continuous_screenings DROP CONSTRAINT continuous_screenings_object_or_opensanction_check;

ALTER TABLE continuous_screenings ALTER COLUMN object_type SET NOT NULL;
ALTER TABLE continuous_screenings ALTER COLUMN object_id SET NOT NULL;
ALTER TABLE continuous_screenings ALTER COLUMN object_internal_id SET NOT NULL;

ALTER TABLE continuous_screenings DROP COLUMN opensanction_entity_id;
ALTER TABLE continuous_screenings DROP COLUMN opensanction_entity_payload;

-- +goose StatementEnd
