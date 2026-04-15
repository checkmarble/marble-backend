-- +goose Up

create index idx_unscored_scores
    on scoring_scores (org_id, record_type, created_at)
    include (record_id)
    where source = 'initial' and deleted_at is null;

-- +goose Down

drop index idx_unscored_scores;
