-- +goose NO TRANSACTION

-- +goose Up
drop index concurrently idx_screening_match_comments_screening_match_id;
create index concurrently idx_screening_match_comments_screening_match_id
    on screening_match_comments (screening_match_id)
    where screening_match_id is not null;

create index concurrently idx_screening_match_comments_continuous_screening_match_id
    on screening_match_comments (continuous_screening_match_id)
    where continuous_screening_match_id is not null;

-- +goose Down
drop index concurrently idx_screening_match_comments_continuous_screening_match_id;

drop index concurrently idx_screening_match_comments_screening_match_id;
create index concurrently idx_screening_match_comments_screening_match_id
    on screening_match_comments (screening_match_id);
