package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

// Implement GlobalCollector interface for stub global collector
type AppVersionCollector struct {
	apiVersion string
}

func NewAppVersionCollector(apiVersion string) GlobalCollector {
	return AppVersionCollector{
		apiVersion: apiVersion,
	}
}

func (c AppVersionCollector) Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	return []models.MetricData{models.NewGlobalMetric(AppVersionMetricName, nil, &c.apiVersion, from, to)}, nil
}

type PostgresVersionRepository interface {
	GetPostgresVersion(ctx context.Context, exec repositories.Executor) (int, error)
}

type PostgresVersionCollector struct {
	executorFactory executor_factory.ExecutorFactory
	repository      PostgresVersionRepository
}

func NewPostgresVersionCollector(repository PostgresVersionRepository, executorFactory executor_factory.ExecutorFactory) GlobalCollector {
	return PostgresVersionCollector{
		executorFactory: executorFactory,
		repository:      repository,
	}
}

func (c PostgresVersionCollector) Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	version, err := c.repository.GetPostgresVersion(ctx, c.executorFactory.NewExecutor())
	if err != nil {
		return nil, err
	}

	return []models.MetricData{models.NewGlobalMetric(PostgresVersionMetricName, new(float64(version)), nil, from, to)}, nil
}
