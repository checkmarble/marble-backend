package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type ContinuousScreeningMarbleDbRepository interface {
	GetEnabledConfigStableIdsByOrg(ctx context.Context, exec repositories.Executor,
		orgIds []string) (map[string][]uuid.UUID, error)
}

type ContinuousScreeningClientDbRepository interface {
	CountMonitoredObjectsByConfigStableIds(ctx context.Context, exec repositories.Executor,
		configStableIds []uuid.UUID) (int, error)
}

// ContinuousScreeningCollector implements the Collector interface
// This collector tracks the number of monitored objects linked to active continuous screening configs
// It uses a gauge metric to report the current state (not a counter)
type ContinuousScreeningCollector struct {
	marbleDbRepository ContinuousScreeningMarbleDbRepository
	clientDbRepository ContinuousScreeningClientDbRepository
	executorFactory    executor_factory.ExecutorFactory
}

func NewContinuousScreeningCollector(
	marbleDbRepository ContinuousScreeningMarbleDbRepository,
	clientDbRepository ContinuousScreeningClientDbRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return ContinuousScreeningCollector{
		marbleDbRepository: marbleDbRepository,
		clientDbRepository: clientDbRepository,
		executorFactory:    executorFactory,
	}
}

// Collect retrieves the current count of monitored objects for each organization
// This is a gauge metric that represents the current state at collection time
func (c ContinuousScreeningCollector) Collect(
	ctx context.Context,
	orgs []models.Organization,
	from, to time.Time,
) ([]models.MetricData, error) {
	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	// Get enabled config stable IDs for all organizations from Marble DB
	configStableIdsByOrg, err := c.marbleDbRepository.GetEnabledConfigStableIdsByOrg(
		ctx,
		c.executorFactory.NewExecutor(),
		orgIds,
	)
	if err != nil {
		return nil, err
	}

	metrics := make([]models.MetricData, 0, len(orgs))

	// For each organization, count monitored objects in their client database
	for _, org := range orgs {
		configStableIds := configStableIdsByOrg[org.Id.String()]

		// Get the client database executor for this organization
		clientExec, err := c.executorFactory.NewClientDbExecutor(ctx, org.Id)
		if err != nil {
			// Log error but continue with other organizations
			utils.LogAndReportSentryError(ctx, err)
			continue
		}

		count, err := c.clientDbRepository.CountMonitoredObjectsByConfigStableIds(
			ctx,
			clientExec,
			configStableIds,
		)
		if err != nil {
			// Log error but continue with other organizations
			utils.LogAndReportSentryError(ctx, err)
			continue
		}

		// Create a gauge metric with the current count
		metrics = append(metrics, models.NewOrganizationMetric(
			CSMonitoredObjectsMetricName,
			utils.Ptr(float64(count)),
			nil,
			orgMap[org.Id.String()],
			from,
			to,
		))
	}

	return metrics, nil
}
