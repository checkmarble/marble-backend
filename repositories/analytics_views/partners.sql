DROP VIEW IF EXISTS analytics.partners;

CREATE VIEW
      analytics.partners
WITH
      (security_invoker = false) AS
SELECT
      id,
      created_at,
      name
FROM
      marble.partners;