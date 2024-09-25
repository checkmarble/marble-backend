DROP VIEW IF EXISTS analytics.decision_rules;

CREATE VIEW
      analytics.decision_rules
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      decision_id,
      error_code,
      outcome,
      result,
      rule_id,
      score_modifier
FROM
      marble.decision_rules