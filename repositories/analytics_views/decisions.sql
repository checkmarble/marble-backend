CREATE OR REPLACE VIEW analytics.decisions
WITH (security_invoker=false)
AS 
SELECT 
      id,
      org_id,
      created_at,
      outcome,
      scenario_id,
      scenario_name,
      scenario_description,
      scenario_version,
      score,
      error_code,
      deleted_at,
      trigger_object_type,
      scheduled_execution_id,
      case_id
FROM marble.decisions
