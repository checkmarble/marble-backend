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

type DecisionCollectorRepository interface {
	CountDecisionsByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
		from, to time.Time) (map[string]int, error)
}

// Implement Collector interface for decision collector
// This collector counts the number of decisions made by an organization
type DecisionCollector struct {
	decisionRepository DecisionCollectorRepository

	executorFactory executor_factory.ExecutorFactory
}

func NewDecisionCollector(decisionRepository DecisionCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return DecisionCollector{
		decisionRepository: decisionRepository,
		executorFactory:    executorFactory,
	}
}

func (c DecisionCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(periods))

	for _, period := range periods {
		orgDecisionCounts, err := c.decisionRepository.CountDecisionsByOrg(ctx,
			c.executorFactory.NewExecutor(), orgIds, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, count := range orgDecisionCounts {
			metrics = append(metrics, models.NewOrganizationMetric(DecisionCountMetricName,
				utils.Ptr(float64(count)), nil, orgMap[orgId], period.From, period.To),
			)
		}
	}

	return metrics, nil
}
