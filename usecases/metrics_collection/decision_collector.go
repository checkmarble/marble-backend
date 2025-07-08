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
	CountDecisions(ctx context.Context, exec repositories.Executor, orgIds []string,
		from, to time.Time) (map[string]int, error)
}

// Implement Collector interface for decision collector
// This collector counts the number of decisions made by an organization
type DecisionCollector struct {
	decisionRepository DecisionCollectorRepository

	executorFactory executor_factory.ExecutorFactory
	frequency       models.MetricCollectionFrequency
}

func NewDecisionCollector(decisionRepository DecisionCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return DecisionCollector{
		decisionRepository: decisionRepository,
		executorFactory:    executorFactory,
		frequency:          models.MetricCollectionFrequencyDaily,
	}
}

func (c DecisionCollector) Collect(ctx context.Context, orgIds []string, from, to time.Time) ([]models.MetricData, error) {
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.Frequency(c.frequency))
	if err != nil {
		return nil, err
	}

	metrics := make([]models.MetricData, 0, len(orgIds)*len(periods))

	for _, period := range periods {
		orgDecisionCounts, err := c.decisionRepository.CountDecisions(ctx,
			c.executorFactory.NewExecutor(), orgIds, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, count := range orgDecisionCounts {
			metrics = append(metrics, models.NewOrganizationMetric("decision.count",
				utils.Ptr(float64(count)), nil, orgId, &period.From, &period.To,
				c.frequency),
			)
		}
	}

	return metrics, nil
}
