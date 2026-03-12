-- +goose Up

alter table scoring_rulesets
    add column scoring_interval_seconds int not null default 15552000;

-- Index used to optimize the query to find batches of stale scores.
-- Most scores will come from ruleset evaluation, so we optimize for that
-- case where we filter by created_at (+ used for ordering).
create index idx_stale_scores
    on scoring_scores (org_id, record_type, created_at)
    include (record_id)
    where deleted_at is null;

-- +goose Down

alter table scoring_rulesets
    drop column scoring_interval_seconds;

drop index idx_stale_scores;
