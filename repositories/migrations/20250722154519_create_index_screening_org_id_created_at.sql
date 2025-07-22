-- +goose NO TRANSACTION
-- +goose Up

create index concurrently if not exists idx_sc_org_id on sanction_checks (org_id, created_at desc) include (status);

-- +goose Down

drop index if exists idx_sc_org_id;
