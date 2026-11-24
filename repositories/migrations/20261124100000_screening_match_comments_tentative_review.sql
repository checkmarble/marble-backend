-- +goose Up
-- +goose StatementBegin

alter table screening_match_comments
    add column tentative_review jsonb,
    add column superseded_at timestamptz;

alter table screening_match_comments
    add constraint screening_match_comments_tentative_review_status
        check (tentative_review is null or tentative_review->>'status' in ('confirmed_hit', 'no_hit'));

create unique index screening_match_comments_active_tentative_review
    on screening_match_comments (screening_match_id)
    where tentative_review is not null and superseded_at is null and screening_match_id is not null;

create unique index continuous_screening_match_comments_active_tentative_review
    on screening_match_comments (continuous_screening_match_id)
    where tentative_review is not null and superseded_at is null and continuous_screening_match_id is not null;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop index continuous_screening_match_comments_active_tentative_review;

drop index screening_match_comments_active_tentative_review;

alter table screening_match_comments
    drop constraint screening_match_comments_tentative_review_status;

alter table screening_match_comments
    drop column superseded_at,
    drop column tentative_review;

-- +goose StatementEnd
