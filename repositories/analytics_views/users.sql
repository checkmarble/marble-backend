DROP VIEW IF EXISTS analytics.users;

CREATE VIEW
      analytics.users
WITH
      (security_invoker = false) AS
SELECT
      id,
      organization_id,
      email,
      role,
      deleted_at
FROM
      marble.users