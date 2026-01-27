-- +goose Up
-- +goose NO TRANSACTION
create index concurrently idx_cs_matches_screening_entity 
    on continuous_screening_matches (continuous_screening_id, opensanction_entity_id);

-- +goose Down
drop index idx_cs_matches_screening_entity;
