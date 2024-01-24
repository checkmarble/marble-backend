DROP VIEW IF EXISTS analytics.cases;

CREATE VIEW
      analytics.cases
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      'placeholder' AS name,
      status,
      created_at,
      inbox_id
FROM
      marble.cases