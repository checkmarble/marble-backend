-- +goose Up
-- +goose StatementBegin
create table screening_freeform_searches (
    id uuid primary key,
    org_id uuid not null,
    user_id uuid null,
    api_key_id uuid null,
    provider text not null,
    created_at timestamp with time zone not null default now(),
    search_input jsonb not null,
    result jsonb,

    constraint fk_screening_freeform_searches_org foreign key (org_id) references organizations (id) on delete cascade,
    constraint fk_screening_freeform_searches_user foreign key (user_id) references users (id) on delete set null,
    constraint fk_screening_freeform_searches_api_key foreign key (api_key_id) references api_keys (id) on delete set null
);

create index idx_screening_freeform_searches_org_created_at on screening_freeform_searches (org_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table screening_freeform_searches;
-- +goose StatementEnd
