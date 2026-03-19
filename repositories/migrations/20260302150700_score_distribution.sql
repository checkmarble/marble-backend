-- +goose Up

create index idx_score_distribution
    on scoring_scores (org_id, record_type, ruleset_id)
    include (risk_level)
    where (source = 'ruleset' and deleted_at is null);

-- +goose Down

drop index idx_score_distribution;
