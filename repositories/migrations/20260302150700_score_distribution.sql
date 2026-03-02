-- +goose Up

create index idx_score_distribution
    on scoring_scores (org_id, entity_type, ruleset_id)
    include (score)
    where (source = 'ruleset' and deleted_at is null);

-- +goose Down

drop index idx_score_distribution;
