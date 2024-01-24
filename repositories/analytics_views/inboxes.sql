DROP VIEW IF EXISTS analytics.inboxes;

CREATE VIEW
      analytics.inboxes
WITH
      (security_invoker = false) AS
SELECT
      id,
      name,
      created_at,
      updated_at,
      organization_id,
      status
FROM
      marble.inboxes