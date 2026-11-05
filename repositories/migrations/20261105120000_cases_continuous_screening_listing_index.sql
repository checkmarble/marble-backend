-- +goose Up
-- +goose NO TRANSACTION

-- Serves ListOrganizationCases when filtering on continuous screening cases, which are rare enough among an
-- organization's cases that the partial index stays small. The predicate must stay identical to that query's filter,
-- otherwise the planner cannot use this partial index.
create index concurrently if not exists idx_cases_continuous_screening_listing
    on cases (org_id, inbox_id, (boost is null), created_at desc, id desc)
    where type = 'continuous_screening';

-- +goose Down
-- +goose NO TRANSACTION

drop index concurrently if exists idx_cases_continuous_screening_listing;
