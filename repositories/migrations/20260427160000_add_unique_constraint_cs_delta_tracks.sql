-- +goose Up
-- +goose StatementBegin
DELETE FROM continuous_screening_delta_tracks a
USING continuous_screening_delta_tracks b
WHERE a.operation IN ('add', 'update')
  AND b.operation IN ('add', 'update')
  AND a.org_id = b.org_id
  AND a.object_type = b.object_type
  AND a.object_id = b.object_id
  AND a.object_internal_id = b.object_internal_id
  AND (a.created_at < b.created_at OR (a.created_at = b.created_at AND a.id < b.id));

CREATE UNIQUE INDEX idx_cs_delta_tracks_unique_add_update
ON continuous_screening_delta_tracks (org_id, object_type, object_id, object_internal_id)
WHERE operation IN ('add', 'update');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_cs_delta_tracks_unique_add_update;
-- +goose StatementEnd
