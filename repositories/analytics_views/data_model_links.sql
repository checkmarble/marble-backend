DROP VIEW IF EXISTS analytics.data_model_links;

CREATE VIEW
      analytics.data_model_links
WITH
      (security_invoker = false) AS
SELECT
      id,
      organization_id,
      name,
      parent_table_id,
      parent_field_id,
      child_table_id,
      child_field_id
FROM
      marble.data_model_links