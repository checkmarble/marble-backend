package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

type MonitoredObjectActiveClientDbRepository interface {
	IsContinuousScreeningSetup(ctx context.Context, exec repositories.Executor) (bool, error)
	CountActiveMonitoredObjects(ctx context.Context, exec repositories.Executor,
		yearStart, yearEnd time.Time) (int, error)
}

type MonitoredObjectActiveCollector struct {
	clientDbRepository MonitoredObjectActiveClientDbRepository
	executorFactory    executor_factory.ExecutorFactory
}

func NewMonitoredObjectActiveCollector(
	clientDbRepository MonitoredObjectActiveClientDbRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return MonitoredObjectActiveCollector{
		clientDbRepository: clientDbRepository,
		executorFactory:    executorFactory,
	}
}

func (c MonitoredObjectActiveCollector) Collect(
	ctx context.Context,
	orgs []models.Organization,
	from, to time.Time,
) ([]models.MetricData, error) {
	_, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	yearStart := time.Date(to.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	metrics := make([]models.MetricData, 0, len(orgs))

	for _, org := range orgs {
		clientExec, err := c.executorFactory.NewClientDbExecutor(ctx, org.Id)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
			continue
		}

		isSetup, err := c.clientDbRepository.IsContinuousScreeningSetup(ctx, clientExec)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
			continue
		}
		if !isSetup {
			continue
		}

		count, err := c.clientDbRepository.CountActiveMonitoredObjects(ctx, clientExec, yearStart, to)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
			continue
		}

		metrics = append(metrics, models.NewOrganizationMetric(
			CSMonitoredObjectsMetricName,
			utils.Ptr(float64(count)),
			nil,
			orgMap[org.Id.String()],
			yearStart,
			to,
		))
	}

	return metrics, nil
}

