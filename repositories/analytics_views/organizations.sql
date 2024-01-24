DROP VIEW IF EXISTS analytics.organizations;

CREATE VIEW
      analytics.organizations
WITH
      (security_invoker = false) AS
SELECT
      id,
      name,
      database_name,
      deleted_at,
      export_scheduled_execution_s3
FROM
      marble.organizations