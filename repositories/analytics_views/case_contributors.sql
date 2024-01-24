DROP VIEW IF EXISTS analytics.case_contributors;

CREATE VIEW
      analytics.case_contributors
WITH
      (security_invoker = false) AS
SELECT
      id,
      case_id,
      user_id,
      created_at
FROM
      marble.case_contributors;