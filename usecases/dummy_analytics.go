package usecases

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type DummyAnalyticsUsecase struct {
	analyticsFactory executor_factory.AnalyticsExecutorFactory
}

type AnalyticsResult struct {
	TotalCount int
}

func (uc DummyAnalyticsUsecase) Do(ctx context.Context, orgId string, start, end time.Time) (AnalyticsResult, error) {
	exec, err := uc.analyticsFactory.GetExecutor(ctx)
	if err != nil {
		return AnalyticsResult{}, err
	}

	var result AnalyticsResult

	query := squirrel.Select("count()").
		From(uc.analyticsFactory.BuildTarget("staging/decisions")).
		Where("outcome = ?", "block_and_review").
		Where("created_at between ? and ?", start, end)

	query = uc.analyticsFactory.BuildPushdownFilter(query, orgId, start, end)
	sql, args, err := query.ToSql()
	if err != nil {
		return AnalyticsResult{}, err
	}

	rows := exec.QueryRowContext(ctx, sql, args...)

	if err := rows.Scan(&result.TotalCount); err != nil {
		return AnalyticsResult{}, err
	}

	return result, rows.Err()
}
