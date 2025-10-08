package executor_factory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
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
	ddbOnce         sync.Once
	exporterDdbOnce sync.Once

	db       repositories.AnalyticsExecutor
	exportDb repositories.AnalyticsExecutor
)

func (f AnalyticsExecutorFactory) GetExecutor(ctx context.Context) (repositories.AnalyticsExecutor, error) {
	var err error

	ddbOnce.Do(func() {
		var ddb *duckdb.Connector

		ddb, err = duckdb.NewConnector("", nil)
		if err != nil {
			return
		}

		db = repositories.NewDuckDbExecutor(sql.OpenDB(ddb))

		switch f.config.Type {
		case infra.BlobTypeS3, infra.BlobTypeGCS:
			_, err = db.ExecContext(ctx, fmt.Sprintf(`create secret if not exists analytics (%s);`, f.config.ConnectionString))
		}
	})

	if err != nil {
		return nil, err
	}

	return db, nil
}

func (f AnalyticsExecutorFactory) GetExecutorWithSource(ctx context.Context, alias string) (repositories.AnalyticsExecutor, error) {
	var err error

	exporterDdbOnce.Do(func() {
		exportDb, err = f.GetExecutor(ctx)
		if err != nil {
			return
		}

		if _, err = exportDb.ExecContext(ctx, f.buildUpstreamAttachStatement(alias)); err != nil {
			return
		}
		if _, err = exportDb.ExecContext(ctx, fmt.Sprintf(`set threads to 1; call postgres_execute('%[1]s', 'set enable_seqscan = 0'); call postgres_execute('%[1]s', 'set enable_indexscan = 0');`, alias)); err != nil {
			return
		}
	})

	if err != nil {
		// DuckDB exposes sensitive data in its error messages, so for now we sanitize it
		return nil, errors.New("could not connect analytics connector to PostgreSQL [redacted error]")
	}

	return exportDb, nil
}

func (f AnalyticsExecutorFactory) BuildTarget(table string, triggerObject *string, aliases ...string) string {
	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	tr := "*"
	if triggerObject != nil {
		tr = "trigger_object_type=" + *triggerObject
	}

	return fmt.Sprintf(`read_parquet('%s/*/*/*/%s/*.parquet', hive_partitioning = true, union_by_name = true) %s`, f.BuildTablePrefix(table), tr, pgx.Identifier.Sanitize([]string{alias}))
}

func (f AnalyticsExecutorFactory) BuildTablePrefix(table string) string {
	return fmt.Sprintf(`%s/%s`, f.config.Bucket, table)
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

	or := squirrel.Or{}

	if firstBetweenYears != end.Year() && start.Year() != end.Year() {
		betweens := make([]int, end.Year()-firstBetweenYears)

		for y := range end.Year() - firstBetweenYears {
			betweens[y] = firstBetweenYears + y
		}

		or = append(or, squirrel.Expr(fmt.Sprintf("%s in ?", pgx.Identifier.Sanitize([]string{alias, "year"})), betweens))
	}

	if start.Year() == end.Year() {
		or = append(or, squirrel.Expr(
			fmt.Sprintf("%s = ? and %s between ? and ?", pgx.Identifier.Sanitize([]string{alias, "year"}), pgx.Identifier.Sanitize([]string{alias, "month"})),
			start.Year(), start.Month(), end.Month()))
	} else {
		or = append(or, squirrel.Or{
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

	query = query.Where(or)

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

	for _, f := range filters.Fields {
		switch f.Source {
		case models.AnalyticsSourceTriggerObject:
			f.Field = analytics.TriggerObjectFieldPrefix + f.Field
		case models.AnalyticsSourceIngestedData:
			f.Field = analytics.DatabaseFieldPrefix + f.Field
		}

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

func (f AnalyticsExecutorFactory) buildUpstreamAttachStatement(alias string) string {
	dsn, err := url.Parse(f.config.PgConfig.ConnectionString)

	if f.config.PgConfig.ConnectionString == "" || err != nil {
		dsn = &url.URL{}
		q := url.Values{}

		if strings.HasPrefix(f.config.PgConfig.Hostname, "/") {
			q.Set("host", f.config.PgConfig.Hostname)
			f.config.PgConfig.Hostname = "localhost"
		}

		dsn.Scheme = "postgres"
		dsn.Host = f.config.PgConfig.Hostname + ":" + f.config.PgConfig.Port
		dsn.Path = "/" + f.config.PgConfig.Database
		dsn.User = url.UserPassword(f.config.PgConfig.User, f.config.PgConfig.Password)

		if f.config.PgConfig.SslMode != "" {
			q.Set("sslmode", f.config.PgConfig.SslMode)
		}

		dsn.RawQuery = q.Encode()
	}

	return fmt.Sprintf(
		`attach or replace '%s' as %s (type postgres, read_only)`,
		dsn.String(),
		alias,
	)
}
