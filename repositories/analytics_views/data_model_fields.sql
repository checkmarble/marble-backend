DROP VIEW IF EXISTS analytics.data_model_fields;

CREATE VIEW
      analytics.data_model_fields
WITH
      (security_invoker = false) AS
SELECT
      id,
      table_id,
      name,
type,
nullable,
description,
is_enum
FROM
      marble.data_model_fields