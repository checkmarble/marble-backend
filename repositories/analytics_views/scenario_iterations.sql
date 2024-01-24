DROP VIEW IF EXISTS analytics.scenario_iterations;

CREATE VIEW
      analytics.scenario_iterations
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      scenario_id,
      version,
      created_at,
      updated_at,
      score_review_threshold,
      score_reject_threshold,
      deleted_at,
      schedule
FROM
      marble.scenario_iterations