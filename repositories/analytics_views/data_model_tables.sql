DROP VIEW IF EXISTS analytics.data_model_tables;

CREATE VIEW
      analytics.data_model_tables
WITH
      (security_invoker = false) AS
SELECT
      id,
      organization_id,
      name,
      description
FROM
      marble.data_model_tables