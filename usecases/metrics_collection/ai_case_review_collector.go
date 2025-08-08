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

type AiCaseReviewCollectorRepository interface {
	CountAiCaseReviewsByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
		from, to time.Time) (map[string]int, error)
}

// Implement Collector interface for AI case review collector
// This collector counts the number of ai case reviews made by an organization
type AiCaseReviewCollector struct {
	aiCaseReviewRepository AiCaseReviewCollectorRepository

	executorFactory executor_factory.ExecutorFactory
}

func NewAiCaseReviewCollector(aiCaseReviewRepository AiCaseReviewCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return AiCaseReviewCollector{
		aiCaseReviewRepository: aiCaseReviewRepository,
		executorFactory:        executorFactory,
	}
}

// Collect AI case reviews count by organization by daily frequency period
func (c AiCaseReviewCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(periods))

	for _, period := range periods {
		orgAiCaseReviewCounts, err := c.aiCaseReviewRepository.CountAiCaseReviewsByOrg(ctx,
			c.executorFactory.NewExecutor(), orgIds, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, count := range orgAiCaseReviewCounts {
			metrics = append(
				metrics,
				models.NewOrganizationMetric(
					AiCaseReviewCountMetricName,
					utils.Ptr(float64(count)),
					nil,
					orgMap[orgId],
					period.From,
					period.To,
				),
			)
		}
	}

	return metrics, nil
}
