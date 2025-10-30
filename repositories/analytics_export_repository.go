package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/utils"
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

func AnalyticsGetLatestRow(ctx context.Context, exec AnalyticsExecutor, table string) (uuid.UUID, time.Time, error) {
	query := squirrel.Select("id", "created_at").From(table).OrderBy("created_at desc, id desc").Limit(1)
	sql, _, err := query.ToSql()
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}

	row := exec.QueryRowContext(ctx, sql)

	var (
		id        uuid.UUID
		createdAt time.Time
	)

	if err := row.Scan(&id, &createdAt); err != nil {
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
		inner = inner.Where("(d.created_at, d.id) > (?::timestamp with time zone, ?)", req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
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

	switch nRows {
	case 0:
		utils.LoggerFromContext(ctx).DebugContext(ctx, "decisions export is up to date")
	default:
		utils.LoggerFromContext(ctx).DebugContext(ctx, "decisions export succeeded", "rows", nRows)
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
		).
		From("marble.decisions d").
		Where("d.org_id = ?", req.OrgId).
		Where("d.trigger_object_type = ?", req.TriggerObject).
		Where("d.created_at < ?", req.EndTime).
		OrderBy("d.created_at, d.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		innerInner = innerInner.Where("(d.created_at, d.id) > (?::timestamp with time zone, ?)", req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	inner := squirrel.
		Select(
			"dr.id",
			"dr.decision_id",
			"dr.score_modifier",
			"dr.result",
			"dr.outcome",
			"dr.rule_id",
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

	switch nRows {
	case 0:
		utils.LoggerFromContext(ctx).DebugContext(ctx, "decision rules export is up to date")
	default:
		utils.LoggerFromContext(ctx).DebugContext(ctx, "decision rules export succeeded", "rows", nRows)
	}

	return int(nRows), nil
}

func AnalyticsCopyScreenings(ctx context.Context, exec AnalyticsExecutor, req AnalyticsCopyRequest) (int, error) {
	inner := squirrel.
		Select(
			"sc.id",
			"min(sc.decision_id::text)::uuid as decision_id",
			"min(sc.status) as status",
			"min(scc.stable_id::text)::uuid as screening_config_id",
			"min(scc.name) as screening_name",
			"count(*) filter (where scm.id is not null) as matches",
			"min(s.id::text)::uuid as scenario_id",
			"min(sc.created_at) as created_at",
			"min(sc.org_id::text)::uuid as org_id",
			"extract(year from min(sc.created_at))::int as year", "extract(month from min(sc.created_at))::int as month",
			"min(s.trigger_object_type) as trigger_object_type",
		).
		From("marble.screenings sc").
		InnerJoin("marble.screening_configs scc on scc.id = sc.screening_config_id").
		LeftJoin("marble.screening_matches scm on scm.screening_id = sc.id").
		InnerJoin("marble.scenario_iterations si on si.id = scc.scenario_iteration_id").
		InnerJoin("marble.scenarios s on s.id = si.scenario_id").
		InnerJoin("marble.decisions d on d.id = sc.decision_id").
		Where("sc.org_id = ?", req.OrgId).
		Where("s.trigger_object_type = ?", req.TriggerObject).
		Where("sc.created_at < ?", req.EndTime).
		GroupBy("sc.id").
		OrderBy("min(sc.created_at), sc.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		inner = inner.Where("(sc.created_at, sc.id) > (?::timestamp with time zone, ?)", req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

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

	switch nRows {
	case 0:
		utils.LoggerFromContext(ctx).DebugContext(ctx, "screenings export is up to date")
	default:
		utils.LoggerFromContext(ctx).DebugContext(ctx, "screenings export succeeded", "rows", nRows)
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
		return b.Column(fmt.Sprintf(`(min(d.trigger_object::text)::jsonb->>'%s')::%s as "%s%s"`, field.Name, sqlType, analytics.TriggerObjectFieldPrefix, field.Name))
	} else {
		return b.Column(fmt.Sprintf(`(d.trigger_object->>'%s')::%s as "%s%s"`, field.Name, sqlType, analytics.TriggerObjectFieldPrefix, field.Name))
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
		return b.Column(fmt.Sprintf(`(min(d.analytics_fields::text)::jsonb->>'%s')::%s as "%s%s"`, field.Name, sqlType, analytics.DatabaseFieldPrefix, field.Name))
	} else {
		return b.Column(fmt.Sprintf(`(d.analytics_fields->>'%s')::%s as "%s%s"`, field.Name, sqlType, analytics.DatabaseFieldPrefix, field.Name))
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
