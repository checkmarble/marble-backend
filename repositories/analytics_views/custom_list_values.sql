DROP VIEW IF EXISTS analytics.custom_list_values;

CREATE VIEW
      analytics.custom_list_values
WITH
      (security_invoker = false) AS
SELECT
      id,
      custom_list_id,
      created_at,
      deleted_at
FROM
      marble.custom_list_values