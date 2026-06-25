-- +goose Up
-- +goose StatementBegin
-- Reuse the existing screening_match_comments table for continuous screening matches by adding an
-- alternate FK. Exactly one of screening_match_id / continuous_screening_match_id must be set.
alter table screening_match_comments
    alter column screening_match_id drop not null;

alter table screening_match_comments
    add column continuous_screening_match_id uuid;

alter table screening_match_comments
    add constraint fk_continuous_screening_match
        foreign key (continuous_screening_match_id)
        references continuous_screening_matches (id) on delete cascade;

alter table screening_match_comments
    add constraint screening_match_comments_one_match_ref
        check (num_nonnulls(screening_match_id, continuous_screening_match_id) = 1);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOTE: The downgrade could fail if there are existing comments referencing continuous screening matches.
-- Delete the comments attached to continuous screening matches before dropping the column
alter table screening_match_comments
    drop constraint screening_match_comments_one_match_ref;

alter table screening_match_comments
    drop constraint fk_continuous_screening_match;

alter table screening_match_comments
    drop column continuous_screening_match_id;

alter table screening_match_comments
    alter column screening_match_id set not null;
-- +goose StatementEnd
