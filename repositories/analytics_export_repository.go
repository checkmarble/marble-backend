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

func AnalyticsGetLatestRow(ctx context.Context, exec AnalyticsExecutor, orgId, triggerObjectType, table string) (uuid.UUID, time.Time, error) {
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

func AnalyticsCopyScreenings(ctx context.Context, exec AnalyticsExecutor, req AnalyticsCopyRequest) (int, error) {
	innerInner := squirrel.
		Select(
			"sc.id",
			"s.id as scenario_id",
			"s.trigger_object_type",
			"sc.org_id",
			"sc.decision_id",
			"sc.screening_config_id",
			"sc.status",
			"sc.created_at",
		).
		From("marble.screenings sc").
		InnerJoin("marble.screening_configs scc on scc.id = sc.screening_config_id").
		InnerJoin("marble.scenario_iterations si on si.id = scc.scenario_iteration_id").
		InnerJoin("marble.scenarios s on s.id = si.scenario_id").
		Where("sc.org_id = ?", req.OrgId).
		Where("s.trigger_object_type = ?", req.TriggerObject).
		Where("sc.created_at < ?", req.EndTime).
		OrderBy("sc.created_at, sc.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		innerInner = innerInner.Where("(sc.created_at, sc.id) > (?::timestamp with time zone, ?)",
			req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	inner := squirrel.
		Select(
			"sc.id",
			"min(sc.decision_id::text)::uuid as decision_id",
			"min(sc.status) as status",
			"min(scc.stable_id::text)::uuid as screening_config_id",
			"min(scc.name) as screening_name",
			"(select count(*) from screening_matches m where m.screening_id = sc.id) as matches",
			"min(sc.scenario_id::text)::uuid as scenario_id",
			"min(sc.created_at) as created_at",
			"min(sc.org_id::text)::uuid as org_id",
			"extract(year from min(sc.created_at))::int as year",
			"extract(month from min(sc.created_at))::int as month",
			"min(sc.trigger_object_type) as trigger_object_type",
		).
		FromSelect(innerInner, "sc").
		InnerJoin("marble.screening_configs scc on scc.id = sc.screening_config_id").
		InnerJoin("marble.decisions d on d.id = sc.decision_id").
		Where("sc.org_id = ?", req.OrgId).
		Where("sc.trigger_object_type = ?", req.TriggerObject).
		Where("sc.created_at < ?", req.EndTime).
		GroupBy("sc.id").
		OrderBy("created_at, id")

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, true)
	}
	for _, f := range req.ExtraDbFields {
		inner = analyticsAddExtraField(inner, f, true)
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
			"dr.rule_id as rule_id",
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
