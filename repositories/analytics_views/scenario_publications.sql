DROP VIEW IF EXISTS analytics.scenario_publications;

CREATE VIEW
      analytics.scenario_publications
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      rank,
      scenario_id,
      scenario_iteration_id,
      publication_action,
      created_at
FROM
      marble.scenario_publications