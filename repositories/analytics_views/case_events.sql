DROP VIEW IF EXISTS analytics.case_events;

CREATE VIEW
      analytics.case_events
WITH
      (security_invoker = false) AS
SELECT
      id,
      case_id,
      user_id,
      event_type,
      created_at,
      resource_id,
      resource_type,
      new_value,
      previous_value
FROM
      marble.case_events