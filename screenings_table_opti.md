## Current schema

```sql
create table screenings
(
    id                   uuid                     default uuid_generate_v4() not null
        constraint sanction_checks_pkey
            primary key,
    decision_id          uuid                                                not null,
    status               text                     default 'in_review'::text  not null
        constraint screenings_status_check
            check (status = ANY (ARRAY ['confirmed_hit'::text, 'no_hit'::text, 'in_review'::text, 'error'::text])),
    search_input         jsonb,
    search_datasets      text[],
    match_threshold      integer                                             not null,
    match_limit          integer                                             not null,
    is_manual            boolean                  default false,
    is_partial           boolean                  default false,
    is_archived          boolean                  default false,
    initial_has_matches  boolean                  default false              not null,
    requested_by         uuid
        constraint fk_screening_user
            references users,
    created_at           timestamp with time zone default now(),
    updated_at           timestamp with time zone default now(),
    whitelisted_entities text[]                   default '{}'::text[]       not null,
    error_codes          text[],
    screening_config_id  uuid                                                not null
        constraint fk_screening_config
            references screening_configs,
    initial_query        jsonb,
    org_id               uuid                                                not null,
    number_of_matches    integer
)
```

## Usage

Updated:

- status (~5-10%)
- is_archived (< 1%)
- updated_at (whenever any other is changed)

## Excessively denormalized

- search_datasets
- match_threshold/limit ? (prob no because may use org's default, but it's just two numbers)
- whitelisted_entities
