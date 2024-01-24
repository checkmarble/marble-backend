DROP VIEW IF EXISTS analytics.apikeys;

CREATE VIEW
      analytics.apikeys
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      deleted_at,
      role
FROM
      marble.apikeys;