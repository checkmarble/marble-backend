package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

type FreeformSearchCollectorRepository interface {
	CountFreeformSearchesByProvider(ctx context.Context, exec repositories.Executor, orgIds []string,
		providers []models.ScreeningProvider, from, to time.Time,
	) (models.ByOrgByProviderCounter, error)
}

// Implement Collector interface for freeform search collector
// This collector counts the number of freeform searches created by provider and by org
type FreeformSearchByProviderCollector struct {
	repository      FreeformSearchCollectorRepository
	executorFactory executor_factory.ExecutorFactory
	providers       []models.ScreeningProvider
}

func NewFreeformSearchByProviderCollector(
	repository FreeformSearchCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
	providers []models.ScreeningProvider,
) Collector {
	return FreeformSearchByProviderCollector{
		repository:      repository,
		executorFactory: executorFactory,
		providers:       providers,
	}
}

// Collect freeform searches count by organization and provider by daily frequency period
func (c FreeformSearchByProviderCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	exec := c.executorFactory.NewExecutor()
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(c.providers)*len(periods))

	for _, period := range periods {
		orgFreeformCounts, err := c.repository.CountFreeformSearchesByProvider(ctx,
			exec, orgIds, c.providers, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, orgCount := range orgFreeformCounts {
			for provider, providerCount := range orgCount {
				metricName, err := buildFreeformSearchMetricName(models.ScreeningProvider(provider))
				if err != nil {
					// Should never happen
					return nil, err
				}
				metrics = append(metrics, models.NewOrganizationMetric(metricName,
					utils.Ptr(float64(providerCount)), nil, orgMap[orgId], period.From, period.To),
				)
			}
		}
	}

	return metrics, nil
}
