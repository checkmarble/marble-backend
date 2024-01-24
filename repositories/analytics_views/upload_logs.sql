DROP VIEW IF EXISTS analytics.upload_logs;

CREATE VIEW
      analytics.upload_logs
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      user_id,
      file_name,
      status,
      started_at,
      finished_at,
      lines_processed,
      table_name
FROM
      marble.upload_logs