-- +goose NO TRANSACTION
-- +goose Up

create index concurrently idx_sanction_check_whitelists_entity_id on sanction_check_whitelists (org_id, entity_id);

-- +goose Down

drop index idx_sanction_check_whitelists_entity_id;