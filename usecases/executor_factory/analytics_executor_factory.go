package executor_factory

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/infra"
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
		db.ExecContext(ctx, fmt.Sprintf(`create secret if not exists analytics (%s);`, f.config.ConnectionString))
	})

	if err != nil {
		return nil, err
	}

	return db, nil
}

func (f AnalyticsExecutorFactory) BuildTarget(table string) string {
	return fmt.Sprintf(`read_parquet('%s/%s/*/*/*/*.parquet', hive_partitioning = true, union_by_name = true)`, f.config.Bucket, table)
}

func (f AnalyticsExecutorFactory) BuildPushdownFilter(query squirrel.SelectBuilder, orgId string, start, end time.Time) squirrel.SelectBuilder {
	if end.Before(start) {
		return query
	}

	firstBetweenYears := start.Year() + 1

	if firstBetweenYears != end.Year() && start.Year() != end.Year() {
		betweens := make([]int, end.Year()-firstBetweenYears)

		for y := range end.Year() - firstBetweenYears {
			betweens[y] = firstBetweenYears + y
		}

		query = query.Where("year in ?", betweens)
	}

	if start.Year() == end.Year() {
		query = query.Where("year = ? and month between ? and ?", start.Year(), start.Month(), end.Month())
	} else {
		query = query.Where(squirrel.Or{
			squirrel.And{
				squirrel.Eq{"year": start.Year()},
				squirrel.Expr("month between ? and 12", start.Month()),
			},
			squirrel.And{
				squirrel.Eq{"year": end.Year()},
				squirrel.Expr("month between 1 and ?", end.Month()),
			},
		})
	}

	return query
}
