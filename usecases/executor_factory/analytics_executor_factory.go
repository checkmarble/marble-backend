package executor_factory

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5"
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

		// NB for future reference: we tried with cache_httpfs, which for our disk-less containers was a net drag on performance
		// compared to the native httpfs cache. We will come back to this in the future to give an option for deployments of
		// Marble that have a persistent disk. It should then be setup at connection dial and configured here.
		ddb, err = duckdb.NewConnector("", func(execer driver.ExecerContext) error {
			_, err := execer.ExecContext(ctx, `set threads = $1;`, []driver.NamedValue{
				{
					// We use a number of threads higher than the number of CPUs to account for the fact that, on the volumes we have been testing,
					// the response time is IO rather than CPU bound. An even higher value may even make sense.
					Value:   utils.GetEnv("DUCKDB_THREADS", runtime.NumCPU()*4),
					Ordinal: 1,
				},
			})
			return err
		})
		if err != nil {
			return
		}

		sqlDb := sql.OpenDB(ddb)
		// This is essentially saying that I want no concurrency on the analytics connector, because the cache (which we want to hit consistently)
		// is per-connection.
		// This could also be tuned differently if using cache_httpfs with a disk.
		sqlDb.SetMaxOpenConns(utils.GetEnv("DUCKDB_MAX_OPEN_CONNS", 1))

		db = repositories.NewDuckDbExecutor(sqlDb)

		switch f.config.Type {
		case infra.BlobTypeS3, infra.BlobTypeGCS:
			_, err = db.ExecContext(ctx, fmt.Sprintf(`
				create secret if not exists analytics (%s);
			`, f.config.ConnectionString))
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
		if _, err = exportDb.ExecContext(ctx, `set threads to 1;`); err != nil {
			return
		}
	})

	if err != nil {
		// DuckDB exposes sensitive data in its error messages, so for now we sanitize it
		return nil, errors.New("could not connect analytics connector to PostgreSQL [redacted error]")
	}

	return exportDb, nil
}

func (f AnalyticsExecutorFactory) BuildTarget(table string, orgId string, triggerObjectType string, aliases ...string) string {
	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	return fmt.Sprintf(`read_parquet('%s/org_id=%s/year=*/month=*/trigger_object_type=%s/*.parquet', hive_partitioning = true, union_by_name = true) %s`,
		f.BuildTablePrefix(table),
		orgId,
		triggerObjectType,
		pgx.Identifier.Sanitize([]string{alias}))
}

func (f AnalyticsExecutorFactory) BuildTablePrefix(table string) string {
	return fmt.Sprintf(`%s/%s`, f.config.Bucket, table)
}

func (f AnalyticsExecutorFactory) BuildPushdownFilter(query squirrel.SelectBuilder, orgId string,
	start, end time.Time, triggerObjectType string, aliases ...string,
) squirrel.SelectBuilder {
	// Align time range on UTC to select the proper list of partitions
	start, end = start.UTC(), end.UTC()

	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	if end.Before(start) {
		return query
	}

	query = query.
		Where(fmt.Sprintf("%s = ?", pgx.Identifier.Sanitize([]string{alias, "org_id"})), orgId).
		Where(fmt.Sprintf("%s = ?", pgx.Identifier.Sanitize([]string{alias, "trigger_object_type"})), triggerObjectType)

	firstBetweenYears := start.Year() + 1

	or := squirrel.Or{}

	if firstBetweenYears != end.Year() && start.Year() != end.Year() {
		betweens := make([]int, end.Year()-firstBetweenYears)

		for y := range end.Year() - firstBetweenYears {
			betweens[y] = firstBetweenYears + y
		}

		or = append(or, squirrel.Expr(fmt.Sprintf("%s in ?",
			pgx.Identifier.Sanitize([]string{alias, "year"})), betweens))
	}

	if start.Year() == end.Year() {
		or = append(or, squirrel.Expr(
			fmt.Sprintf("%s = ? and %s between ? and ?",
				pgx.Identifier.Sanitize([]string{alias, "year"}),
				pgx.Identifier.Sanitize([]string{alias, "month"})),
			start.Year(), start.Month(), end.Month()))
	} else {
		or = append(or, squirrel.Or{
			squirrel.And{
				squirrel.Eq{pgx.Identifier.Sanitize([]string{alias, "year"}): start.Year()},
				squirrel.Expr(fmt.Sprintf("%s between ? and 12",
					pgx.Identifier.Sanitize([]string{alias, "month"})), start.Month()),
			},
			squirrel.And{
				squirrel.Eq{pgx.Identifier.Sanitize([]string{alias, "year"}): end.Year()},
				squirrel.Expr(fmt.Sprintf("%s between 1 and ?",
					pgx.Identifier.Sanitize([]string{alias, "month"})), end.Month()),
			},
		})
	}

	query = query.Where(or)

	return query
}

func (f AnalyticsExecutorFactory) ApplyFilters(query squirrel.SelectBuilder,
	scenario models.Scenario, filters dto.AnalyticsQueryFilters, aliases ...string,
) (squirrel.SelectBuilder, error) {
	alias := "main"
	if len(aliases) > 0 {
		alias = aliases[0]
	}

	query = f.BuildPushdownFilter(query, scenario.OrganizationId, filters.Start, filters.End, scenario.TriggerObjectType, aliases...)
	query = query.Where(fmt.Sprintf("%s = ?",
		pgx.Identifier.Sanitize([]string{alias, "scenario_id"})), filters.ScenarioId)

	if len(filters.ScenarioVersions) > 0 {
		query = query.Where(fmt.Sprintf("%s in ?",
			pgx.Identifier.Sanitize([]string{alias, "version"})), filters.ScenarioVersions)
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

	// We cannot control how DuckDB will create connections from the pool or
	// transactions, so we need to set query options on the connections
	// directly.
	q := dsn.Query()
	q.Set("options", `-cenable_seqscan=0`)
	dsn.RawQuery = q.Encode()

	return fmt.Sprintf(
		`attach or replace '%s' as %s (type postgres, read_only)`,
		dsn.String(),
		alias,
	)
}
