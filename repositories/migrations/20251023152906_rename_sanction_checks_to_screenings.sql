-- +goose Up
-- +goose StatementBegin
-- Drop all the screening views
DROP VIEW IF EXISTS screening_whitelists;

DROP VIEW IF EXISTS screening_files;

DROP VIEW IF EXISTS screening_match_comments;

DROP VIEW IF EXISTS screening_matches;

DROP VIEW IF EXISTS screening_configs;

DROP VIEW IF EXISTS screenings;

-- Rename tables
ALTER TABLE sanction_check_match_comments
RENAME TO screening_match_comments;

ALTER TABLE sanction_check_matches
RENAME TO screening_matches;

ALTER TABLE sanction_check_files
RENAME TO screening_files;

ALTER TABLE sanction_checks
RENAME TO screenings;

ALTER TABLE sanction_check_configs
RENAME TO screening_configs;

ALTER TABLE sanction_check_whitelists
RENAME TO screening_whitelists;

-- Rename columns in screening_matches
ALTER TABLE screening_matches
RENAME COLUMN sanction_check_id TO screening_id;

-- Rename columns in screening_match_comments
ALTER TABLE screening_match_comments
RENAME COLUMN sanction_check_match_id TO screening_match_id;

-- Rename columns in screening_files
ALTER TABLE screening_files
RENAME COLUMN sanction_check_id TO screening_id;

-- Rename columns in screenings
ALTER TABLE screenings
RENAME COLUMN sanction_check_config_id TO screening_config_id;

-- Rename indexes
ALTER INDEX idx_sanction_checks_decision_id
RENAME TO idx_screenings_decision_id;

ALTER INDEX idx_sanction_check_matches_sanction_check_id
RENAME TO idx_screening_matches_screening_id;

ALTER INDEX idx_sanction_check_match_comments_sanction_check_match_id
RENAME TO idx_screening_match_comments_screening_match_id;

ALTER INDEX idx_sanction_check_files_sanction_check_id
RENAME TO idx_screening_files_screening_id;

ALTER INDEX idx_sanction_check_whitelist
RENAME TO idx_screening_whitelist;

ALTER INDEX idx_sc_org_id
RENAME TO idx_screenings_org_id;

ALTER INDEX idx_sanction_check_whitelists_entity_id
RENAME TO idx_screening_whitelists_entity_id;

ALTER INDEX idx_scc_iteration_id
RENAME TO idx_screening_configs_iteration_id;

-- Rename constraints
ALTER TABLE screening_matches
RENAME CONSTRAINT fk_sanction_check TO fk_screening;

ALTER TABLE screening_match_comments
RENAME CONSTRAINT fk_sanction_check_match TO fk_screening_match;

ALTER TABLE screening_files
RENAME CONSTRAINT fk_sanction_check_match TO fk_screening_file;

ALTER TABLE screenings
RENAME CONSTRAINT fk_sanction_check_config TO fk_screening_config;

ALTER TABLE screenings
RENAME CONSTRAINT sanction_checks_status_check TO screenings_status_check;

ALTER TABLE screening_configs
RENAME CONSTRAINT fk_scenario_iteration TO fk_screening_config_scenario_iteration;

-- Rename foreign key constraints to users
ALTER TABLE screenings
RENAME CONSTRAINT fk_user TO fk_screening_user;

ALTER TABLE screening_matches
RENAME CONSTRAINT fk_user TO fk_screening_match_user;

ALTER TABLE screening_match_comments
RENAME CONSTRAINT fk_user TO fk_screening_match_comment_user;

ALTER TABLE screening_whitelists
RENAME CONSTRAINT fk_user TO fk_screening_whitelist_user;

-- Rename foreign key constraints to organizations
ALTER TABLE screening_whitelists
RENAME CONSTRAINT fk_organization TO fk_screening_whitelist_organization;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Rename constraints back
ALTER TABLE screening_whitelists
RENAME CONSTRAINT fk_screening_whitelist_organization TO fk_organization;

ALTER TABLE screening_whitelists
RENAME CONSTRAINT fk_screening_whitelist_user TO fk_user;

ALTER TABLE screening_match_comments
RENAME CONSTRAINT fk_screening_match_comment_user TO fk_user;

ALTER TABLE screening_matches
RENAME CONSTRAINT fk_screening_match_user TO fk_user;

ALTER TABLE screenings
RENAME CONSTRAINT fk_screening_user TO fk_user;

ALTER TABLE screening_configs
RENAME CONSTRAINT fk_screening_config_scenario_iteration TO fk_scenario_iteration;

ALTER TABLE screenings
RENAME CONSTRAINT screenings_status_check TO sanction_checks_status_check;

ALTER TABLE screenings
RENAME CONSTRAINT fk_screening_config TO fk_sanction_check_config;

ALTER TABLE screening_files
RENAME CONSTRAINT fk_screening_file TO fk_sanction_check_match;

ALTER TABLE screening_match_comments
RENAME CONSTRAINT fk_screening_match TO fk_sanction_check_match;

ALTER TABLE screening_matches
RENAME CONSTRAINT fk_screening TO fk_sanction_check;

-- Rename indexes back
ALTER INDEX idx_screening_configs_iteration_id
RENAME TO idx_scc_iteration_id;

ALTER INDEX idx_screening_whitelists_entity_id
RENAME TO idx_sanction_check_whitelists_entity_id;

ALTER INDEX idx_screenings_org_id
RENAME TO idx_sc_org_id;

ALTER INDEX idx_screening_whitelist
RENAME TO idx_sanction_check_whitelist;

ALTER INDEX idx_screening_files_screening_id
RENAME TO idx_sanction_check_files_sanction_check_id;

ALTER INDEX idx_screening_match_comments_screening_match_id
RENAME TO idx_sanction_check_match_comments_sanction_check_match_id;

ALTER INDEX idx_screening_matches_screening_id
RENAME TO idx_sanction_check_matches_sanction_check_id;

ALTER INDEX idx_screenings_decision_id
RENAME TO idx_sanction_checks_decision_id;

-- Rename columns back
ALTER TABLE screenings
RENAME COLUMN screening_config_id TO sanction_check_config_id;

ALTER TABLE screening_files
RENAME COLUMN screening_id TO sanction_check_id;

ALTER TABLE screening_match_comments
RENAME COLUMN screening_match_id TO sanction_check_match_id;

ALTER TABLE screening_matches
RENAME COLUMN screening_id TO sanction_check_id;

-- Rename tables back
ALTER TABLE screening_whitelists
RENAME TO sanction_check_whitelists;

ALTER TABLE screening_configs
RENAME TO sanction_check_configs;

ALTER TABLE screenings
RENAME TO sanction_checks;

ALTER TABLE screening_files
RENAME TO sanction_check_files;

ALTER TABLE screening_matches
RENAME TO sanction_check_matches;

ALTER TABLE screening_match_comments
RENAME TO sanction_check_match_comments;

-- Recreate the views
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