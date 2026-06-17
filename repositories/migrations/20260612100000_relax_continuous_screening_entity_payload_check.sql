-- +goose Up
-- +goose StatementBegin

-- The OpenSanctions entity payload of a (dataset-triggered) continuous screening can now be
-- offloaded to blob storage, in which case the `opensanction_entity_payload` column is left NULL.
-- Relax the check constraint so it no longer requires the payload to be present: an entity id is
-- enough to identify the dataset-triggered branch.
ALTER TABLE continuous_screenings DROP CONSTRAINT continuous_screenings_object_or_opensanction_check;

ALTER TABLE continuous_screenings ADD CONSTRAINT continuous_screenings_object_or_opensanction_check
CHECK (
    (object_type IS NOT NULL AND object_id IS NOT NULL AND object_internal_id IS NOT NULL)
    OR
    (opensanction_entity_id IS NOT NULL)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE continuous_screenings DROP CONSTRAINT continuous_screenings_object_or_opensanction_check;

-- Rows whose entity payload was offloaded (and thus stored as NULL) while the constraint was
-- relaxed would violate the stricter constraint below. Backfill them with an empty JSON object so
-- the constraint can be recreated.
UPDATE continuous_screenings
SET opensanction_entity_payload = '{}'::jsonb
WHERE opensanction_entity_id IS NOT NULL
    AND opensanction_entity_payload IS NULL;

ALTER TABLE continuous_screenings ADD CONSTRAINT continuous_screenings_object_or_opensanction_check
CHECK (
    (object_type IS NOT NULL AND object_id IS NOT NULL AND object_internal_id IS NOT NULL)
    OR
    (opensanction_entity_id IS NOT NULL AND opensanction_entity_payload IS NOT NULL)
);

-- +goose StatementEnd
