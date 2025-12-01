package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type AnalyticsCopyRequest struct {
	OrgId     string
	Watermark *models.Watermark
	Table     string

	TriggerObject       string
	TriggerObjectFields []models.Field
	ExtraDbFields       []models.Field

	EndTime time.Time
	// TODO: Limit is problematic, both to backfill previoud data, and because we risk incomplete syncs.
	// We would still like a limit for the initial sync, otherwise it risks going over the allotted time period.
	Limit int
}

func AnalyticsGetLatestRow(ctx context.Context, exec AnalyticsExecutor,
	orgId, triggerObjectType, table string,
) (uuid.UUID, time.Time, error) {
	query := squirrel.
		Select("id", "created_at").
		From(table).
		Where("org_id = ?", orgId).
		Where("trigger_object_type = ?", triggerObjectType).
		OrderBy("created_at desc, id desc").
		Limit(1)

	querySql, args, err := query.ToSql()
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}

	row := exec.QueryRowContext(ctx, querySql, args...)

	var (
		id        uuid.UUID
		createdAt time.Time
	)

	if err := row.Scan(&id, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) || IsDuckDBNoFilesError(err) {
			return uuid.Nil, time.Time{}, nil
		}

		return uuid.Nil, time.Time{}, err
	}

	return id, createdAt, nil
}

func AnalyticsCopyDecisions(ctx context.Context, exec AnalyticsExecutor, req AnalyticsCopyRequest) (int, error) {
	inner := squirrel.
		Select(
			"d.id", "d.score", "d.outcome",
			"s.id as scenario_id", "s.name as scenario_name",
			"si.version",
			"d.created_at",
			"d.org_id",
			"extract(year from d.created_at)::int as year", "extract(month from d.created_at)::int as month",
			"d.trigger_object_type",
		).
		From("marble.decisions d").
		InnerJoin("marble.scenario_iterations si on si.id = d.scenario_iteration_id").
		InnerJoin("marble.scenarios s on s.id = si.scenario_id").
		Where("d.org_id = ?", req.OrgId).
		Where("d.trigger_object_type = ?", req.TriggerObject).
		Where("d.created_at < ?", req.EndTime).
		OrderBy("d.created_at, d.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		inner = inner.Where("(d.created_at, d.id) > (?::timestamp with time zone, ?)",
			req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, false)
	}
	for _, f := range req.ExtraDbFields {
		inner = analyticsAddExtraField(inner, f, false)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	unsafeQuery, err := unsafeBuildSqlQuery(innerSql, args)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( select * from postgres_query(?, ?) ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, req.Table)

	result, err := exec.ExecContext(ctx, query, "pg", unsafeQuery)
	if err != nil {
		return 0, err
	}
	nRows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(nRows), nil
}

func AnalyticsCopyDecisionRules(ctx context.Context, exec AnalyticsExecutor, req AnalyticsCopyRequest) (int, error) {
	innerInner := squirrel.
		Select(
			"d.id",
			"d.scenario_iteration_id",
			"d.pivot_id", "d.pivot_value",
			"d.created_at",
			"extract(year from d.created_at)::int as year", "extract(month from d.created_at)::int as month",
			"d.trigger_object_type",
			"d.trigger_object",
			"d.analytics_fields",
		).
		From("marble.decisions d").
		Where("d.org_id = ?", req.OrgId).
		Where("d.trigger_object_type = ?", req.TriggerObject).
		Where("d.created_at < ?", req.EndTime).
		OrderBy("d.created_at, d.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		innerInner = innerInner.Where("(d.created_at, d.id) > (?::timestamp with time zone, ?)",
			req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	inner := squirrel.
		Select(
			"dr.id",
			"dr.decision_id",
			"dr.score_modifier",
			"dr.result",
			"dr.outcome",
			"dr.rule_id",
			"sir.stable_rule_id as stable_rule_id",
			"sir.name as rule_name",
			"d.pivot_id", "d.pivot_value",
			"s.id as scenario_id",
			"s.name as scenario_name",
			"si.version",
			"d.created_at",
			"dr.org_id",
			"extract(year from d.created_at)::int as year", "extract(month from d.created_at)::int as month",
			"d.trigger_object_type",
		).
		FromSelect(innerInner, "d").
		InnerJoin("marble.decision_rules dr on dr.decision_id = d.id").
		InnerJoin("marble.scenario_iterations si on si.id = d.scenario_iteration_id").
		InnerJoin("marble.scenarios s on s.id = si.scenario_id").
		LeftJoin("marble.scenario_iteration_rules sir on sir.id = dr.rule_id").
		OrderBy("d.created_at, d.id")

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, false)
	}
	for _, f := range req.ExtraDbFields {
		inner = analyticsAddExtraField(inner, f, false)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	unsafeQuery, err := unsafeBuildSqlQuery(innerSql, args)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( select * from postgres_query(?, ?) ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, req.Table)

	result, err := exec.ExecContext(ctx, query, "pg", unsafeQuery)
	if err != nil {
		return 0, err
	}
	nRows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(nRows), nil
}

func generateScreeningsExportQuery(req AnalyticsCopyRequest, limitOverride *int, endOverride *time.Time) (squirrel.SelectBuilder, error) {
	if limitOverride != nil {
		req.Limit = *limitOverride
	}
	if endOverride != nil && endOverride.Before(req.EndTime) {
		req.EndTime = *endOverride
	}

	ctes := WithCtes("configs", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		return b.
			Select(
				"scc.id AS screening_config_id",
				"scc.stable_id",
				"scc.name",
				"s.id AS scenario_id",
				"s.trigger_object_type",
			).
			From("scenarios AS s").
			InnerJoin("scenario_iterations AS si ON si.scenario_id = s.id").
			InnerJoin("screening_configs AS scc ON scc.scenario_iteration_id = si.id").
			Where("s.org_id = ?", req.OrgId).
			Where("s.trigger_object_type = ?", req.TriggerObject)
	}).
		With("screenings_by_config", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
			inner := squirrel.
				Select(
					"scs.id",
					"scs.decision_id",
					"scs.created_at",
					"scs.status",
					"scs.org_id",
					"configs.scenario_id",
					"configs.trigger_object_type").
				From("screenings AS scs").
				Where("scs.screening_config_id = configs.screening_config_id").
				Where("scs.created_at < ?", req.EndTime).
				OrderBy("scs.created_at, scs.id").
				Limit(uint64(req.Limit))

			if req.Watermark != nil {
				inner = inner.Where("(scs.created_at, scs.id) > (?::timestamp with time zone, ?)",
					req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
			}

			innerSql, args, err := inner.ToSql()
			if err != nil {
				// return nil, err
				// TODO: handle this if we decide the general direction of the query is good enough
			}

			// The subtlety in the query resides in the lateral join here. It means the subquery is executed for each row of the outer query.
			return b.
				Select("scs.*").
				Column("configs.name AS config_name").
				Column("configs.stable_id AS config_stable_id").
				From("configs").
				CrossJoin("LATERAL ("+innerSql+") as scs", args...)
		}).
		With("limited_screenings", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
			return b.
				Select("*").
				From("screenings_by_config").
				OrderBy("created_at, id").
				Limit(uint64(req.Limit))
		})

	inner := squirrel.StatementBuilder.
		Select(
			"limited_screenings.id",
			"MIN(limited_screenings.decision_id::text)::uuid AS decision_id",
			"MIN(limited_screenings.status) AS status",
			"MIN(limited_screenings.scenario_id::text)::uuid AS scenario_id",
			"MIN(limited_screenings.created_at) AS created_at",
			"MIN(limited_screenings.org_id::text)::uuid AS org_id",
			"EXTRACT(year FROM MIN(limited_screenings.created_at))::int AS year",
			"EXTRACT(month FROM MIN(limited_screenings.created_at))::int AS month",
			"MIN(limited_screenings.trigger_object_type) AS trigger_object_type",
			"MIN(limited_screenings.config_stable_id::text)::uuid AS screening_config_id",
			"MIN(limited_screenings.config_name) AS screening_name",
			"(SELECT count(*) FROM screening_matches m WHERE m.screening_id = limited_screenings.id) AS matches",
		).
		PrefixExpr(ctes).
		From("limited_screenings").
		InnerJoin("decisions AS d ON d.id = limited_screenings.decision_id").
		GroupBy("limited_screenings.id").
		PlaceholderFormat(squirrel.Dollar)

	return inner, nil
}

// The function does three steps:
//   - check if there are any screenings to export at all
//   - try to export them using a limited size (1 month at most) time window from which to read those screenings, to avoid infinite inflation
//     of screenigns related to distinct configurations as scenario versions are created
//   - if the previous step did not find any, or if the watermark is at zero, just run the most general query (possibly more expensive, but should not be so on a scale
//     that breaks the system)
func AnalyticsCopyScreenings(ctx context.Context, exec AnalyticsExecutor, dbExec Executor, req AnalyticsCopyRequest) (int, error) {
	// First, run the query with limit 1 to see if there are even rows to export. If not, early exit.
	// This is to avoid running large queries in a loop for organizations (or tables) that are used with decisions but not with screenings.
	inner, err := generateScreeningsExportQuery(req, utils.Ptr(1), nil)
	if err != nil {
		return 0, err
	}
	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}
	row := dbExec.QueryRow(ctx, innerSql, args...)
	err = row.Scan()
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// There are no screenings to export, is is useless to run the larger query with possible high load.
		return 0, nil
	case err != nil:
		return 0, err
	}

	// Then, if the watermark is not at zero, try to run the export with an end date at most one month after the watermark.
	// This handles the case where the watermark is not zero so there are screenings to export, but we want to avoid running the query with
	// a time window too large to avoid matching screenings on too many configurations. If we find any:
	// - either we find many, and we got what we wanted for a low cost
	// - or we find few, and we advanced the watermark by at least a day (otherwise, we would be in the case above)
	// - or we find none, and we fall back to running the full query without max end date, at which point we it is run once and the watermark advances,
	//   guaranteeing no doom loop of high load, no progress queries.
	if req.Watermark != nil && !req.Watermark.WatermarkTime.IsZero() {
		end := req.Watermark.WatermarkTime.AddDate(0, 1, 0)
		inner, err = generateScreeningsExportQuery(req, nil, &end)
		if err != nil {
			return 0, err
		}
		num, err := exportScreeningsRun(ctx, exec, inner, req.Table)
		if err != nil {
			return 0, err
		}
		if num > 0 {
			return num, nil
		}
	}

	// Finally, run the export with no limit or end date.
	inner, err = generateScreeningsExportQuery(req, nil, nil)
	if err != nil {
		return 0, err
	}
	return exportScreeningsRun(ctx, exec, inner, req.Table)
}

// utility function that actually runs the export query and returns the number of rows exported. Factorized because we call two versions of it
// with a modified end date.
func exportScreeningsRun(ctx context.Context, exec AnalyticsExecutor, inner squirrel.SelectBuilder, table string) (int, error) {
	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	unsafeQuery, err := unsafeBuildSqlQuery(innerSql, args)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( select * from postgres_query(?, ?) ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, table)

	result, err := exec.ExecContext(ctx, query, "pg", unsafeQuery)
	if err != nil {
		return 0, err
	}
	nRows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(nRows), nil
}

func AnalyticsCopyCaseEvents(ctx context.Context, exec AnalyticsExecutor, req AnalyticsCopyRequest) (int, error) {
	// The deduplicate column adds a rank to every group of rules returned by the
	// query, so we can skip the rules with the maximum value of that column.
	//
	// The last group of rules might be incomplete because of the limit we add,
	// so instead of doing weird subqueries, we use that window function to
	// completely skip the last group, which will be picked up by the next
	// iteration.
	cte := WithCtesRaw("q", func(b squirrel.StatementBuilderType) squirrel.SelectBuilder {
		q1 := b.Select(
			"ce.id",
			"ce.org_id",
			"d.scenario_id",
			"r.stable_rule_id as rule_id",
			"ce.case_id as case_id",
			"ce.new_value as outcome",
			"ce.created_at as created_at",
			"d.trigger_object_type",
			"row_number() over (partition by ce.case_id order by ce.created_at desc) rnk",
			"dense_rank() over (order by ce.created_at, ce.id) as deduplicate",
		).
			From(dbmodels.TABLE_CASE_EVENTS+" ce").
			InnerJoin(dbmodels.TABLE_CASES+" c on c.id = ce.case_id").
			InnerJoin(dbmodels.TABLE_DECISIONS+" d on d.case_id = c.id").
			InnerJoin(dbmodels.TABLE_DECISION_RULES+" dr on dr.decision_id = d.id").
			InnerJoin(dbmodels.TABLE_RULES+" r on r.id = dr.rule_id").
			Where("ce.org_id = ?", req.OrgId).
			Where("ce.event_type = 'outcome_updated'").
			Where("c.status = 'closed'").
			Where("d.trigger_object_type = ?", req.TriggerObject).
			Where("ce.created_at < ?", req.EndTime).
			OrderBy("ce.created_at, ce.id").
			Limit(uint64(req.Limit))

		if req.Watermark != nil {
			q1 = q1.Where("(ce.created_at, ce.id) > (?::timestamp with time zone, ?)",
				req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
		}

		return b.
			Select("*", "max(deduplicate) over() as max_deduplicate").
			FromSelect(q1, "i")
	})

	inner := squirrel.
		Select(
			"q.id",
			"q.scenario_id",
			"q.rule_id",
			"q.case_id",
			"q.outcome",
			"q.created_at",
			"q.org_id",
			"extract(year from q.created_at)::int as year",
			"extract(month from q.created_at)::int as month",
			"q.trigger_object_type",
		).
		From("q").
		PrefixExpr(cte).
		InnerJoin(dbmodels.TABLE_DECISIONS + " d on d.case_id = q.case_id").
		Where("q.rnk = 1 and deduplicate < max_deduplicate")

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, false)
	}
	for _, f := range req.ExtraDbFields {
		inner = analyticsAddExtraField(inner, f, false)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	unsafeQuery, err := unsafeBuildSqlQuery(innerSql, args)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( select * from postgres_query(?, ?) ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, req.Table)

	result, err := exec.ExecContext(ctx, query, "pg", unsafeQuery)
	if err != nil {
		return 0, err
	}
	nRows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(nRows), nil
}

func analyticsAddTriggerObjectField(b squirrel.SelectBuilder, field models.Field, anyValue bool) squirrel.SelectBuilder {
	sqlType := "text"

	switch field.DataType {
	case models.Bool:
		sqlType = "bool"
	case models.Int:
		sqlType = "int"
	case models.Float:
		sqlType = "float"
	case models.Timestamp:
		sqlType = "timestamp with time zone"
	}

	if anyValue {
		return b.Column(fmt.Sprintf(`(min(d.trigger_object::text)::jsonb->>'%s')::%s as "%s%s"`,
			field.Name, sqlType, analytics.TriggerObjectFieldPrefix, field.Name))
	} else {
		return b.Column(fmt.Sprintf(`(d.trigger_object->>'%s')::%s as "%s%s"`, field.Name,
			sqlType, analytics.TriggerObjectFieldPrefix, field.Name))
	}
}

func analyticsAddExtraField(b squirrel.SelectBuilder, field models.Field, anyValue bool) squirrel.SelectBuilder {
	sqlType := "text"

	switch field.DataType {
	case models.Bool:
		sqlType = "bool"
	case models.Int:
		sqlType = "int"
	case models.Float:
		sqlType = "float"
	case models.Timestamp:
		sqlType = "timestamp with time zone"
	}

	if anyValue {
		return b.Column(fmt.Sprintf(`(min(d.analytics_fields::text)::jsonb->>'%s')::%s as "%s%s"`,
			field.Name, sqlType, analytics.DatabaseFieldPrefix, field.Name))
	} else {
		return b.Column(fmt.Sprintf(`(d.analytics_fields->>'%s')::%s as "%s%s"`, field.Name,
			sqlType, analytics.DatabaseFieldPrefix, field.Name))
	}
}

func unsafeBuildSqlQuery(sql string, args []any) (string, error) {
	for _, arg := range args {
		var val string

		switch v := arg.(type) {
		case string:
			val = "'" + strings.ReplaceAll(v, "'", "''") + "'"
		case *string:
			val = "'" + strings.ReplaceAll(*v, "'", "''") + "'"
		case time.Time:
			val = "'" + v.Format(time.RFC3339Nano) + "'"
		case nil:
			val = "NULL"
		default:
			return "", fmt.Errorf("unsupported argument type: %T", v)
		}

		sql = strings.Replace(sql, "?", val, 1)
	}
	return sql, nil
}
