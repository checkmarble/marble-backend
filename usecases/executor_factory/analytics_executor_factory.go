package executor_factory

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgx/v5"
	"github.com/marcboeker/go-duckdb/v2"
)

type AnalyticsExecutorFactory struct {
	config infra.AnalyticsConfig
}

func NewAnalyticsExecutorFactory(config infra.AnalyticsConfig) AnalyticsExecutorFactory {
	return AnalyticsExecutorFactory{
		config: config,
	}
}

var (
	ddbOnce sync.Once
	db      *sql.DB
)

func (f AnalyticsExecutorFactory) GetExecutor(ctx context.Context) (*sql.DB, error) {
	var err error

	ddbOnce.Do(func() {
		var ddb *duckdb.Connector

		ddb, err = duckdb.NewConnector("", nil)
		if err != nil {
			return
		}

		db = sql.OpenDB(ddb)

		_, err = db.ExecContext(ctx, fmt.Sprintf(`create secret if not exists analytics (%s);`, f.config.ConnectionString))
	})

	if err != nil {
		return nil, err
	}

	return db, nil
}

func (f AnalyticsExecutorFactory) BuildTarget(table string, aliases ...string) string {
	alias := "main"
	if len(alias) > 0 {
		alias = aliases[0]
	}

	return fmt.Sprintf(`read_parquet('%s/%s/*/*/*/*/*.parquet', hive_partitioning = true, union_by_name = true) %s`, f.config.Bucket, table, pgx.Identifier.Sanitize([]string{alias}))
}

func (f AnalyticsExecutorFactory) BuildPushdownFilter(query squirrel.SelectBuilder, orgId string, start, end time.Time, triggerObjectType string, aliases ...string) squirrel.SelectBuilder {
	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	if end.Before(start) {
		return query
	}

	query = query.Where(fmt.Sprintf("%s = ?", pgx.Identifier.Sanitize([]string{alias, "trigger_object_type"})), triggerObjectType)

	firstBetweenYears := start.Year() + 1

	if firstBetweenYears != end.Year() && start.Year() != end.Year() {
		betweens := make([]int, end.Year()-firstBetweenYears)

		for y := range end.Year() - firstBetweenYears {
			betweens[y] = firstBetweenYears + y
		}

		query = query.Where(fmt.Sprintf("%s in ?", pgx.Identifier.Sanitize([]string{alias, "year"})), betweens)
	}

	if start.Year() == end.Year() {
		query = query.Where(
			fmt.Sprintf("%s = ? and %s between ? and ?", pgx.Identifier.Sanitize([]string{alias, "year"}), pgx.Identifier.Sanitize([]string{alias, "month"})),
			start.Year(), start.Month(), end.Month())
	} else {
		query = query.Where(squirrel.Or{
			squirrel.And{
				squirrel.Eq{pgx.Identifier.Sanitize([]string{alias, "year"}): start.Year()},
				squirrel.Expr(fmt.Sprintf("%s between ? and 12", pgx.Identifier.Sanitize([]string{alias, "month"})), start.Month()),
			},
			squirrel.And{
				squirrel.Eq{pgx.Identifier.Sanitize([]string{alias, "year"}): end.Year()},
				squirrel.Expr(fmt.Sprintf("%s between 1 and ?", pgx.Identifier.Sanitize([]string{alias, "month"})), end.Month()),
			},
		})
	}

	return query
}

func (f AnalyticsExecutorFactory) ApplyFilters(query squirrel.SelectBuilder, scenario models.Scenario, filters dto.AnalyticsQueryFilters, aliases ...string) (squirrel.SelectBuilder, error) {
	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	query = f.BuildPushdownFilter(query, scenario.OrganizationId, filters.Start, filters.End, scenario.TriggerObjectType, aliases...)
	query = query.Where(fmt.Sprintf("%s = ?", pgx.Identifier.Sanitize([]string{alias, "scenario_id"})), filters.ScenarioId)

	if len(filters.ScenarioVersions) > 0 {
		query = query.Where(fmt.Sprintf("%s in ?", pgx.Identifier.Sanitize([]string{alias, "version"})), filters.ScenarioVersions)
	}

	for _, f := range filters.Trigger {
		lhs, rhs, err := f.ToPredicate(aliases...)
		if err != nil {
			return query, err
		}

		switch f.Op {
		case "in":
			query = query.Where(lhs, rhs)
		default:
			query = query.Where(lhs, rhs...)
		}
	}

	return query, nil
}
