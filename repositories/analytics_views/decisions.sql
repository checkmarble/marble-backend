DROP VIEW IF EXISTS analytics.decisions;

CREATE VIEW
      analytics.decisions
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      created_at,
      outcome,
      scenario_id,
      scenario_iteration_id,
      scenario_description,
      scenario_name,
      scenario_version,
      score,
      trigger_object_type,
      scheduled_execution_id,
      case_id
FROM
      marble.decisions;