DROP VIEW IF EXISTS analytics.api_keys;

CREATE VIEW
      analytics.api_keys
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      deleted_at,
      partner_id,
      role
FROM
      marble.api_keys;