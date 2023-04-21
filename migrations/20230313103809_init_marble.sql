-- +goose Up
-- +goose StatementBegin
-- create and make default the marble schema
CREATE SCHEMA marble;

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA marble TO marble;

ALTER DATABASE marble
SET search_path TO marble,
  public;

ALTER ROLE marble
SET search_path TO marble,
  public;

-- add UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- organizations
CREATE TABLE organizations(
  id uuid DEFAULT uuid_generate_v4(),
  name VARCHAR NOT NULL,
  database_name VARCHAR NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id)
);

-- data models
CREATE TYPE data_models_status AS ENUM ('validated', 'live', 'deprecated');

CREATE TABLE data_models(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  version VARCHAR NOT NULL,
  status data_models_status NOT NULL,
  tables json NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_data_models_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

-- tokens
CREATE TABLE tokens(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  token VARCHAR NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_tokens_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

-- scenarios table
CREATE TABLE scenarios(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  name VARCHAR NOT NULL,
  description VARCHAR NOT NULL,
  trigger_object_type VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_scenarios_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

CREATE TABLE scenario_iterations(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  scenario_id uuid NOT NULL,
  version smallint NOT NULL,
  trigger_condition json NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  score_review_threshold smallint NOT NULL,
  score_reject_threshold smallint NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_scenario_iterations_scenarios FOREIGN KEY(scenario_id) REFERENCES scenarios(id),
  CONSTRAINT fk_scenario_iterations_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

CREATE TABLE scenario_iteration_rules(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  scenario_iteration_id uuid NOT NULL,
  display_order smallint NOT NULL,
  name text NOT NULL,
  description text NOT NULL,
  score_modifier smallint NOT NULL,
  formula json NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_scenario_iteration_rules_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id),
  CONSTRAINT fk_scenario_iteration_rules_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

ALTER TABLE scenarios
ADD COLUMN live_scenario_iteration_id uuid,
  ADD CONSTRAINT fk_scenarios_live_scenario_iteration FOREIGN KEY(live_scenario_iteration_id) REFERENCES scenario_iterations(id);

-- scenario_publications
CREATE TABLE scenario_publications(
  id uuid DEFAULT uuid_generate_v4(),
  rank SERIAL,
  org_id uuid NOT NULL,
  -- user_id uuid NOT NULL,
  scenario_id uuid NOT NULL,
  scenario_iteration_id uuid NOT NULL,
  publication_action VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  PRIMARY KEY(id),
  CONSTRAINT fk_scenario_publications_org FOREIGN KEY(org_id) REFERENCES organizations(id),
  -- CONSTRAINT fk_scenario_publications_user FOREIGN KEY(user_id) REFERENCES users(id),
  CONSTRAINT fk_scenario_publications_scenario_id FOREIGN KEY(scenario_id) REFERENCES scenarios(id),
  CONSTRAINT fk_scenario_publications_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id)
);

-- decisions
-- Outcomes
CREATE TYPE decision_outcome AS ENUM (
  'approve',
  'decline',
  'review',
  'null',
  'unknown'
);

-- decisions table
CREATE TABLE decisions(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  outcome decision_outcome NOT NULL,
  scenario_id uuid NOT NULL,
  scenario_name VARCHAR NOT NULL,
  scenario_description VARCHAR NOT NULL,
  scenario_version INT NOT NULL,
  score INT NOT NULL,
  error_code INT NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  --error_message VARCHAR NOT NULL,
  PRIMARY KEY(id),
  CONSTRAINT fk_decisions_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

-- decision rules table
CREATE TABLE decision_rules(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  decision_id uuid NOT NULL,
  name VARCHAR NOT NULL,
  description VARCHAR NOT NULL,
  score_modifier INT NOT NULL,
  result BOOLEAN NOT NULL,
  error_code INT NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  --error_message VARCHAR,
  PRIMARY KEY(id),
  CONSTRAINT fk_decision_rules_org FOREIGN KEY(org_id) REFERENCES organizations(id),
  CONSTRAINT fk_decision_rules_decisions FOREIGN KEY(decision_id) REFERENCES decisions(id)
);

CREATE TABLE transactions(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  value double precision,
  title VARCHAR,
  description VARCHAR,
  bank_account_id uuid,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id)
);

CREATE TABLE bank_accounts(
  id uuid DEFAULT uuid_generate_v4(),
  object_id VARCHAR NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  balance double precision,
  name VARCHAR,
  currency VARCHAR NOT NULL,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id)
);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS marble CASCADE;

-- +goose StatementEnd