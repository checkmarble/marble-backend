DROP VIEW IF EXISTS analytics.organizations_schema;

CREATE VIEW
      analytics.organizations_schema
WITH
      (security_invoker = false) AS
SELECT
      id,
      org_id AS organization_id,
      schema_name
FROM
      marble.organizations_schema