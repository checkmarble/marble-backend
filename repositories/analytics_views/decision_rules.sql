DROP VIEW IF EXISTS analytics.decision_rules;

CREATE VIEW
      analytics.decision_rules
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      decision_id,
      name,
      description,
      score_modifier,
      result,
      error_code,
      deleted_at
FROM
      marble.decision_rules