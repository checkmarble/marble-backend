DROP VIEW IF EXISTS analytics.scheduled_executions;

CREATE VIEW
      analytics.scheduled_executions
WITH
      (security_invoker = false) AS
SELECT
      id,
      organization_id,
      scenario_id,
      scenario_iteration_id,
      status,
      started_at,
      finished_at,
      number_of_created_decisions,
      manual
FROM
      marble.scheduled_executions