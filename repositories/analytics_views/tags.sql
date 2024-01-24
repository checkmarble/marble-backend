DROP VIEW IF EXISTS analytics.tags;

CREATE VIEW
      analytics.tags
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      name,
      color,
      created_at,
      updated_at,
      deleted_at
FROM
      marble.tags