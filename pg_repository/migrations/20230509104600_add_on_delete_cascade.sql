-- +goose Up
-- +goose StatementBegin
ALTER TABLE data_models
DROP CONSTRAINT fk_data_models_org,
ADD CONSTRAINT fk_data_models_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE tokens
DROP CONSTRAINT fk_tokens_org,
ADD CONSTRAINT fk_tokens_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE scenarios
DROP CONSTRAINT fk_scenarios_org,
ADD CONSTRAINT fk_scenarios_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE scenario_iterations
DROP CONSTRAINT fk_scenario_iterations_scenarios,
DROP CONSTRAINT fk_scenario_iterations_org,
ADD CONSTRAINT fk_scenario_iterations_scenarios FOREIGN KEY(scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE,
ADD CONSTRAINT fk_scenario_iterations_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE scenario_iteration_rules
DROP CONSTRAINT fk_scenario_iteration_rules_scenario_iterations,
DROP CONSTRAINT fk_scenario_iteration_rules_org,
ADD CONSTRAINT fk_scenario_iteration_rules_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id) ON DELETE CASCADE,
ADD CONSTRAINT fk_scenario_iteration_rules_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE scenario_publications
DROP CONSTRAINT fk_scenario_publications_org,
DROP CONSTRAINT fk_scenario_publications_scenario_id,
DROP CONSTRAINT fk_scenario_publications_scenario_iterations,
ADD CONSTRAINT fk_scenario_publications_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE,
ADD CONSTRAINT fk_scenario_publications_scenario_id FOREIGN KEY(scenario_id) REFERENCES scenarios(id) ON DELETE CASCADE,
ADD CONSTRAINT fk_scenario_publications_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id) ON DELETE CASCADE;

ALTER TABLE decisions
DROP CONSTRAINT fk_decisions_org,
ADD CONSTRAINT fk_decisions_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE decision_rules
DROP CONSTRAINT fk_decision_rules_org,
DROP CONSTRAINT fk_decision_rules_decisions,
ADD CONSTRAINT fk_decision_rules_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE,
ADD CONSTRAINT fk_decision_rules_decisions FOREIGN KEY(decision_id) REFERENCES decisions(id) ON DELETE CASCADE;

ALTER TABLE scenarios
DROP CONSTRAINT fk_scenarios_live_scenario_iteration,
ADD CONSTRAINT fk_scenarios_live_scenario_iteration FOREIGN KEY(live_scenario_iteration_id) REFERENCES scenario_iterations(id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE data_models
DROP CONSTRAINT fk_data_models_org,
ADD CONSTRAINT fk_data_models_org FOREIGN KEY(org_id) REFERENCES organizations(id);

ALTER TABLE tokens
DROP CONSTRAINT fk_tokens_org,
ADD CONSTRAINT fk_tokens_org FOREIGN KEY(org_id) REFERENCES organizations(id);

ALTER TABLE scenarios
DROP CONSTRAINT fk_scenarios_org,
ADD CONSTRAINT fk_scenarios_org FOREIGN KEY(org_id) REFERENCES organizations(id);

ALTER TABLE scenario_iterations
DROP CONSTRAINT fk_scenario_iterations_scenarios,
DROP CONSTRAINT fk_scenario_iterations_org,
ADD CONSTRAINT fk_scenario_iterations_scenarios FOREIGN KEY(scenario_id) REFERENCES scenarios(id),
ADD CONSTRAINT fk_scenario_iterations_org FOREIGN KEY(org_id) REFERENCES organizations(id);

ALTER TABLE scenario_iteration_rules
DROP CONSTRAINT fk_scenario_iteration_rules_scenario_iterations,
DROP CONSTRAINT fk_scenario_iteration_rules_org,
ADD CONSTRAINT fk_scenario_iteration_rules_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id),
ADD CONSTRAINT fk_scenario_iteration_rules_org FOREIGN KEY(org_id) REFERENCES organizations(id);

ALTER TABLE scenario_publications
DROP CONSTRAINT fk_scenario_publications_org,
DROP CONSTRAINT fk_scenario_publications_scenario_id,
DROP CONSTRAINT fk_scenario_publications_scenario_iterations,
ADD CONSTRAINT fk_scenario_publications_org FOREIGN KEY(org_id) REFERENCES organizations(id),
ADD CONSTRAINT fk_scenario_publications_scenario_id FOREIGN KEY(scenario_id) REFERENCES scenarios(id),
ADD CONSTRAINT fk_scenario_publications_scenario_iterations FOREIGN KEY(scenario_iteration_id) REFERENCES scenario_iterations(id);

ALTER TABLE decisions
DROP CONSTRAINT fk_decisions_org,
ADD CONSTRAINT fk_decisions_org FOREIGN KEY(org_id) REFERENCES organizations(id);

ALTER TABLE decision_rules
DROP CONSTRAINT fk_decision_rules_org,
DROP CONSTRAINT fk_decision_rules_decisions,
ADD CONSTRAINT fk_decision_rules_org FOREIGN KEY(org_id) REFERENCES organizations(id),
ADD CONSTRAINT fk_decision_rules_decisions FOREIGN KEY(decision_id) REFERENCES decisions(id);

ALTER TABLE scenarios
DROP CONSTRAINT fk_scenarios_live_scenario_iteration,
ADD CONSTRAINT fk_scenarios_live_scenario_iteration FOREIGN KEY(live_scenario_iteration_id) REFERENCES scenario_iterations(id);

-- +goose StatementEnd