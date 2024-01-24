DROP VIEW IF EXISTS analytics.case_tags;

CREATE VIEW
      analytics.case_tags
WITH
      (security_invoker = false) AS
SELECT
      id,
      case_id,
      case_tags,
      created_at,
      deleted_at
FROM
      marble.case_tags