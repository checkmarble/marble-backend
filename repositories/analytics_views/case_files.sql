DROP VIEW IF EXISTS analytics.case_files;

CREATE VIEW
      analytics.case_files
WITH
      (security_invoker = false) AS
SELECT
      id,
      created_at,
      case_id,
      bucket_name,
      file_reference
FROM
      marble.case_files