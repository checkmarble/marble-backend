DROP VIEW IF EXISTS analytics.inbox_users;

CREATE VIEW
      analytics.inbox_users
WITH
      (security_invoker = false) AS
SELECT
      id,
      created_at,
      updated_at,
      inbox_id,
      user_id,
      role
FROM
      marble.inbox_users