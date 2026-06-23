-- +goose Up
-- +goose StatementBegin
create table continuous_screening_match_comments (
    id uuid primary key,
    continuous_screening_match_id uuid not null,
    commented_by uuid not null,
    comment text not null default '',
    created_at timestamp with time zone not null default now(),
    constraint fk_continuous_screening_match foreign key (continuous_screening_match_id)
        references continuous_screening_matches (id) on delete cascade,
    constraint fk_user foreign key (commented_by) references users (id)
);

create index idx_cs_match_comments_match_id
    on continuous_screening_match_comments (continuous_screening_match_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table continuous_screening_match_comments;
-- +goose StatementEnd
