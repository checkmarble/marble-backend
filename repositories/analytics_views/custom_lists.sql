DROP VIEW IF EXISTS analytics.custom_lists;

CREATE VIEW
      analytics.custom_lists
WITH
      (security_invoker = false) AS
SELECT
      id,
      organization_id,
      created_at,
      updated_at,
      deleted_at
FROM
      marble.custom_lists