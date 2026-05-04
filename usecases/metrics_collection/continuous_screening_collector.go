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

type ContinuousScreeningCollectorRepository interface {
	CountCSScreeningsByProvider(ctx context.Context, exec repositories.Executor, orgIds []string, providers []models.ScreeningProvider,
		from, to time.Time,
	) (models.ByOrgByProviderCounter, error)
}

type ContinuousScreeningByProviderCollector struct {
	repository      ContinuousScreeningCollectorRepository
	executorFactory executor_factory.ExecutorFactory
	providers       []models.ScreeningProvider
}

func NewContinuousScreeningByProviderCollector(
	repository ContinuousScreeningCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
	providers []models.ScreeningProvider,
) Collector {
	return ContinuousScreeningByProviderCollector{
		repository:      repository,
		executorFactory: executorFactory,
		providers:       providers,
	}
}

func (c ContinuousScreeningByProviderCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	exec := c.executorFactory.NewExecutor()
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(c.providers)*len(periods))

	for _, period := range periods {
		orgCSScreeningCounts, err := c.repository.CountCSScreeningsByProvider(ctx,
			exec, orgIds, c.providers, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, orgCount := range orgCSScreeningCounts {
			for provider, providerCount := range orgCount {
				screeningCountMetricName, err := buildCSScreeningMetricName(models.ScreeningProvider(provider))
				if err != nil {
					// Should never happen
					return nil, err
				}
				metrics = append(metrics, models.NewOrganizationMetric(screeningCountMetricName,
					utils.Ptr(float64(providerCount)), nil, orgMap[orgId], period.From, period.To),
				)
			}
		}
	}

	return metrics, nil
}
