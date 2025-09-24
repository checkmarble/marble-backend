-- +goose Up
-- +goose StatementBegin
CREATE VIEW
    screenings AS
SELECT
    id,
    decision_id,
    status,
    search_input,
    search_datasets,
    match_threshold,
    match_limit,
    is_manual,
    is_partial,
    is_archived,
    initial_has_matches,
    requested_by,
    created_at,
    updated_at,
    whitelisted_entities,
    error_codes,
    sanction_check_config_id as screening_config_id,
    initial_query,
    org_id,
    number_of_matches
FROM
    sanction_checks;

CREATE VIEW
    screening_configs AS
SELECT
    *
FROM
    sanction_check_configs;

CREATE VIEW
    screening_matches AS
SELECT
    id,
    sanction_check_id as screening_id,
    opensanction_entity_id,
    status,
    query_ids,
    payload,
    reviewed_by,
    created_at,
    updated_at,
    counterparty_id,
    enriched
FROM
    sanction_check_matches;

CREATE VIEW
    screening_match_comments AS
SELECT
    id,
    sanction_check_match_id as screening_match_id,
    commented_by,
comment,
created_at
FROM
    sanction_check_match_comments;

CREATE VIEW
    screening_files AS
SELECT
    id,
    sanction_check_id as screening_id,
    bucket_name,
    file_reference,
    file_name,
    created_at
FROM
    sanction_check_files;

CREATE VIEW
    screening_whitelists AS
SELECT
    *
FROM
    sanction_check_whitelists;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP VIEW screenings;

DROP VIEW screening_configs;

DROP VIEW screening_matches;

DROP VIEW screening_match_comments;

DROP VIEW screening_files;

DROP VIEW screening_whitelists;

-- +goose StatementEnd