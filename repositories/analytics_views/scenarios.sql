DROP VIEW IF EXISTS analytics.scenarios;

CREATE VIEW
      analytics.scenarios
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      name,
      description,
      trigger_object_type,
      created_at,
      deleted_at,
      live_scenario_iteration_id
FROM
      marble.scenarios