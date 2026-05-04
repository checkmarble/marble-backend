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

type ScreeningCollectorRepository interface {
	CountScreeningsByOrg(
		ctx context.Context,
		exec repositories.Executor,
		orgIds []string,
		from, to time.Time,
	) (map[string]int, error)

	CountScreeningsByProvider(
		ctx context.Context,
		exec repositories.Executor,
		orgIds []string,
		providers []string,
		from, to time.Time,
	) (models.ByOrgByProviderCounter, error)
}

// Implement Collector interface for screening collector
// This collector counts the number of screenings made by an organization
type ScreeningCollector struct {
	screeningRepository ScreeningCollectorRepository

	executorFactory executor_factory.ExecutorFactory
}

func NewScreeningCollector(screeningRepository ScreeningCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return ScreeningCollector{
		screeningRepository: screeningRepository,
		executorFactory:     executorFactory,
	}
}

// Collect screenings count by organization by daily frequency period
func (c ScreeningCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(periods))

	for _, period := range periods {
		orgScreeningCounts, err := c.screeningRepository.CountScreeningsByOrg(ctx,
			c.executorFactory.NewExecutor(), orgIds, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, count := range orgScreeningCounts {
			metrics = append(metrics, models.NewOrganizationMetric(ScreeningCountMetricName,
				utils.Ptr(float64(count)), nil, orgMap[orgId], period.From, period.To),
			)
		}
	}

	return metrics, nil
}

// Implement Collector interface for screening collector
// This collector counts the number of screenings created by provider and by org
type ScreeningByProviderCollector struct {
	screeningRepository ScreeningCollectorRepository
	executorFactory     executor_factory.ExecutorFactory

	// TODO: use same Enum as screening provider if defined in screening usecase
	providers []string
}

func NewScreeningByProviderCollector(screeningRepository ScreeningCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
	providers []string,
) Collector {
	return ScreeningByProviderCollector{
		screeningRepository: screeningRepository,
		executorFactory:     executorFactory,
		providers:           providers,
	}
}

// Collect screenings count by organization by daily frequency period
func (c ScreeningByProviderCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	exec := c.executorFactory.NewExecutor()
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(c.providers)*len(periods))

	for _, period := range periods {
		orgScreeningCounts, err := c.screeningRepository.CountScreeningsByProvider(ctx,
			exec, orgIds, c.providers, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, orgCount := range orgScreeningCounts {
			for provider, providerCount := range orgCount {
				screeningCountMetricName, err := buildScreeningMetricName(provider)
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
