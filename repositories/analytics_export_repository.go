package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type AnalyticsCopyRequest struct {
	OrgId     string
	Watermark *models.Watermark
	Table     string

	TriggerObject       string
	TriggerObjectFields []models.Field

	Limit int
}

func AnalyticsGetLatestRow(ctx context.Context, exec AnalyticsExecutor, table string) (uuid.UUID, time.Time, error) {
	query := squirrel.Select("id", "created_at").From(table).OrderBy("created_at desc").Limit(1)
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
			"extract(year from d.created_at) as year", "extract(month from d.created_at) as month",
			"d.trigger_object_type",
		).
		From("pg.marble.decisions d").
		InnerJoin("pg.marble.scenario_iterations si on si.id = d.scenario_iteration_id").
		InnerJoin("pg.marble.scenarios s on s.id = si.scenario_id").
		Where("d.org_id = ?", req.OrgId).
		Where("d.trigger_object_type = ?", req.TriggerObject).
		OrderBy("d.created_at, d.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		inner = inner.Where("(d.created_at, d.id::text) > (?::timestamp with time zone, ?)", req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, false)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( %s ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, innerSql, req.Table)
	result, err := exec.ExecContext(ctx, query, args...)
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
			"extract(year from d.created_at) as year", "extract(month from d.created_at) as month",
			"d.trigger_object_type",
		).
		From("pg.marble.decision_rules dr").
		LeftJoin("pg.marble.decisions d on d.id = dr.decision_id").
		InnerJoin("pg.marble.scenario_iterations si on si.id = d.scenario_iteration_id").
		InnerJoin("pg.marble.scenarios s on s.id = si.scenario_id").
		LeftJoin("pg.marble.scenario_iteration_rules sir on sir.id = dr.rule_id").
		Where("d.org_id = ?", req.OrgId).
		Where("d.trigger_object_type = ?", req.TriggerObject).
		OrderBy("d.created_at, dr.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		inner = inner.Where("(d.created_at, dr.id::text) > (?::timestamp with time zone, ?)", req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, false)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( %s ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, innerSql, req.Table)
	result, err := exec.ExecContext(ctx, query, args...)
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
			"any_value(sc.decision_id) as decision_id",
			"any_value(sc.status) as status",
			"any_value(scc.stable_id) as screening_config_id",
			"any_value(scc.name) as screening_name",
			"count() filter (where scm.id is not null) as matches",
			"any_value(s.id) as scenario_id",
			"any_value(sc.created_at) as created_at",
			"any_value(sc.org_id) as org_id",
			"extract(year from any_value(sc.created_at)) as year", "extract(month from any_value(sc.created_at)) as month",
			"any_value(s.trigger_object_type) as trigger_object_type",
		).
		From("pg.marble.sanction_checks sc").
		InnerJoin("pg.marble.sanction_check_configs scc on scc.id = sc.sanction_check_config_id").
		LeftJoin("pg.marble.sanction_check_matches scm on scm.sanction_check_id = sc.id").
		InnerJoin("pg.marble.scenario_iterations si on si.id = scc.scenario_iteration_id").
		InnerJoin("pg.marble.scenarios s on s.id = si.scenario_id").
		InnerJoin("pg.marble.decisions d on d.id = sc.decision_id").
		Where("sc.org_id = ?", req.OrgId).
		Where("s.trigger_object_type = ?", req.TriggerObject).
		GroupBy("sc.id").
		OrderBy("any_value(sc.created_at), sc.id").
		Limit(uint64(req.Limit))

	if req.Watermark != nil {
		inner = inner.Where("(sc.created_at, sc.id::text) > (?::timestamp with time zone, ?)", req.Watermark.WatermarkTime, req.Watermark.WatermarkId)
	}

	for _, f := range req.TriggerObjectFields {
		inner = analyticsAddTriggerObjectField(inner, f, true)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`copy ( %s ) to '%s' (format parquet, compression zstd, partition_by (org_id, year, month, trigger_object_type), append)`, innerSql, req.Table)
	result, err := exec.ExecContext(ctx, query, args...)
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
		return b.Column(fmt.Sprintf("(any_value(d.trigger_object)->>'%s')::%s as tr_%s", field.Name, sqlType, field.Name))
	} else {
		return b.Column(fmt.Sprintf("(d.trigger_object->>'%s')::%s as tr_%s", field.Name, sqlType, field.Name))
	}
}
