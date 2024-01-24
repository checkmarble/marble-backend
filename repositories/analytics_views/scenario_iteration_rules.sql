DROP VIEW IF EXISTS analytics.scenario_iteration_rules;

CREATE VIEW
      analytics.scenario_iteration_rules
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      scenario_iteration_id,
      name,
      description,
      score_modifier,
      created_at,
      deleted_at
FROM
      marble.scenario_iteration_rules