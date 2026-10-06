package repositories

import (
	"context"
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
)

// A single statement gives every total, creation date and historical delta the
// same MVCC snapshot. Only reduced aggregates leave PostgreSQL.
func (repo *MarbleDbRepository) Dashboard(ctx context.Context, exec Executor, months int) (models.Dashboard, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Dashboard{}, err
	}

	var payload []byte
	if err := exec.QueryRow(ctx, dashboardQuery, months).Scan(&payload); err != nil {
		return models.Dashboard{}, errors.Wrap(err, "read dashboard metrics")
	}

	var dashboard dbmodels.DbDashboard
	if err := json.Unmarshal(payload, &dashboard); err != nil {
		return models.Dashboard{}, errors.Wrap(err, "decode dashboard metrics")
	}

	return dbmodels.AdaptDashboard(dashboard), nil
}

// Coverage is tracked per entity and per measure in audit.dashboard_coverage:
// All at an instant needs every lifecycle event from then on, New over an
// interval needs every creation event inside it.
//
// Licences are never deleted and keep the created_at set by the database at
// insert, so their history comes from that column and is complete: they feed the
// same lifecycle and recency CTEs as creations, with unbounded coverage.
const dashboardQuery = `
WITH observation AS MATERIALIZED (
    SELECT clock_timestamp() AS observed_at
), settings AS MATERIALIZED (
    SELECT observed_at,
      ((observed_at AT TIME ZONE 'UTC') - make_interval(months => $1)) AT TIME ZONE 'UTC' AS range_start
    FROM observation
), entities AS (
    SELECT unnest(ARRAY['organizations','tenants','users','licenses']) AS entity
), coverage AS MATERIALIZED (
    SELECT entity,
      max(since) FILTER (WHERE measure = 'all') AS all_since,
      max(since) FILTER (WHERE measure = 'new') AS new_since
    FROM audit.dashboard_coverage GROUP BY entity
    UNION ALL SELECT 'licenses', '-infinity'::timestamptz, '-infinity'::timestamptz
), live AS MATERIALIZED (
    -- AllOrganizations lists every retained row, across all environments.
    SELECT 'organizations' AS entity, id AS id, name FROM organizations
    UNION ALL SELECT 'tenants', id, name FROM tenants WHERE deleted_at IS NULL
    UNION ALL SELECT 'users', id,
      coalesce(nullif(trim(concat_ws(' ',first_name,last_name)),''),email)
      FROM users WHERE deleted_at IS NULL
    UNION ALL SELECT 'licenses', id, name FROM licenses
), totals AS (
    SELECT e.entity, count(l.id)::int AS total
    FROM entities e LEFT JOIN live l USING(entity) GROUP BY e.entity
), buckets AS MATERIALIZED (
    SELECT week_start AT TIME ZONE 'UTC' AS start,
      least((week_start + interval '7 days') AT TIME ZONE 'UTC', s.observed_at) AS finish,
      (week_start + interval '7 days') AT TIME ZONE 'UTC' AS closed_finish,
      greatest(week_start AT TIME ZONE 'UTC', s.range_start) AS effective_start,
      s.range_start, s.observed_at
    FROM settings s CROSS JOIN LATERAL generate_series(
      date_trunc('week',s.range_start AT TIME ZONE 'UTC'),
      date_trunc('week',s.observed_at AT TIME ZONE 'UTC'),
      interval '7 days') week_start
), logical_events AS MATERIALIZED (
    -- The former two users triggers emitted identical rows for one operation.
    -- New capture timestamps use clock time per operation, so genuine repeated
    -- transitions in one transaction retain their own observations.
    SELECT DISTINCT ae."table" AS entity, ae.entity_id, ae.operation,
      ae.created_at, ae.data, ae.previous_data
    FROM audit.audit_events ae,settings s
    WHERE ae."table" IN ('organizations','tenants','users')
      AND ae.created_at >= s.range_start AND ae.created_at <= s.observed_at
), lifecycle AS MATERIALIZED (
    SELECT entity,entity_id,operation,created_at,
      CASE
        WHEN operation='INSERT' THEN CASE WHEN entity='organizations' OR data->>'deleted_at' IS NULL THEN 1 ELSE 0 END
        WHEN operation='DELETE' THEN CASE WHEN entity='organizations' OR data->>'deleted_at' IS NULL THEN -1 ELSE 0 END
        WHEN operation='UPDATE' AND entity <> 'organizations' THEN
          (CASE WHEN data->>'deleted_at' IS NULL THEN 1 ELSE 0 END) -
          (CASE WHEN previous_data->>'deleted_at' IS NULL THEN 1 ELSE 0 END)
        ELSE 0
      END AS delta
    FROM logical_events
    UNION ALL SELECT 'licenses', li.id, 'INSERT'::marble.audit_operation, li.created_at, 1
    FROM licenses li, settings s
    WHERE li.created_at >= s.range_start AND li.created_at <= s.observed_at
), weekly AS (
    SELECT t.entity,jsonb_agg(jsonb_build_object(
      'start',b.start,'end',b.finish,
      -- Reverse every later delta from the live total; one missing event after
      -- the observation invalidates it.
      'all',CASE WHEN b.finish = b.observed_at THEN t.total
                WHEN c.all_since IS NOT NULL AND b.finish >= c.all_since
                THEN t.total - coalesce((SELECT sum(delta) FROM lifecycle l WHERE l.entity=t.entity AND l.created_at >= b.finish),0)
                ELSE NULL END,
      -- A week whose coverage starts midweek counts the covered part only and
      -- is flagged partial.
      'new',CASE WHEN c.new_since IS NOT NULL AND c.new_since < b.finish
                THEN (SELECT count(DISTINCT entity_id) FROM lifecycle l
                      WHERE l.entity=t.entity AND l.operation='INSERT'
                        AND l.created_at >= greatest(b.effective_start, c.new_since) AND l.created_at < b.finish)::int
                ELSE NULL END,
      'partial', b.start < b.range_start OR b.finish < b.closed_finish
        OR (c.new_since > b.effective_start AND c.new_since < b.finish)
      ) ORDER BY b.start) AS weeks
    FROM totals t CROSS JOIN buckets b LEFT JOIN coverage c ON c.entity=t.entity GROUP BY t.entity
), known_creation AS MATERIALIZED (
    SELECT l.entity,l.id,l.name,min(ae.created_at) AS created_at
    FROM live l JOIN audit.audit_events ae
      ON ae."table"=l.entity AND ae.entity_id=l.id AND ae.operation='INSERT'
    CROSS JOIN settings s WHERE ae.created_at <= s.observed_at
    GROUP BY l.entity,l.id,l.name
    UNION ALL SELECT l.entity,l.id,l.name,li.created_at
    FROM live l JOIN licenses li ON l.entity='licenses' AND li.id=l.id
    CROSS JOIN settings s WHERE li.created_at <= s.observed_at
), recent_ranked AS (
    SELECT *,row_number() OVER(PARTITION BY entity ORDER BY created_at DESC,id ASC) AS rank
    FROM known_creation
), recent AS (
    SELECT entity,jsonb_agg(jsonb_build_object('id',id,'name',name,'created_at',created_at)
      ORDER BY created_at DESC,id ASC) AS records
    FROM recent_ranked WHERE rank<=3 GROUP BY entity
)
SELECT jsonb_build_object(
    'generated_at',s.observed_at,'months',$1,'range_start',s.range_start,
    'indicators',(SELECT jsonb_object_agg(t.entity,jsonb_build_object(
      -- Unbounded coverage is reported as the range start: complete for every shown week.
      'total',t.total,
      'all_since',CASE WHEN c.all_since = '-infinity' THEN s.range_start ELSE c.all_since END,
      'new_since',CASE WHEN c.new_since = '-infinity' THEN s.range_start ELSE c.new_since END,
      'weeks',w.weeks,'recent',coalesce(r.records,'[]'::jsonb),
      'recent_has_unknown',t.total > (SELECT count(*) FROM known_creation k WHERE k.entity=t.entity)
      )) FROM totals t JOIN weekly w USING(entity) LEFT JOIN recent r USING(entity) LEFT JOIN coverage c USING(entity))
) FROM settings s`
