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

type CaseCollectorRepository interface {
	CountCasesByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
		from, to time.Time) (map[string]int, error)
}

// Implement Collector interface for case collector
// This collector counts the number of cases made by an organization
type CaseCollector struct {
	caseRepository CaseCollectorRepository

	executorFactory executor_factory.ExecutorFactory
}

func NewCaseCollector(caseRepository CaseCollectorRepository,
	executorFactory executor_factory.ExecutorFactory,
) Collector {
	return CaseCollector{
		caseRepository:  caseRepository,
		executorFactory: executorFactory,
	}
}

func (c CaseCollector) Collect(ctx context.Context, orgs []models.Organization, from, to time.Time) ([]models.MetricData, error) {
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyDaily)
	if err != nil {
		return nil, err
	}

	orgIds, orgMap := getOrgIDlistAndPublicIdMap(orgs)

	metrics := make([]models.MetricData, 0, len(orgIds)*len(periods))

	for _, period := range periods {
		orgCaseCounts, err := c.caseRepository.CountCasesByOrg(ctx,
			c.executorFactory.NewExecutor(), orgIds, period.From, period.To)
		if err != nil {
			return nil, err
		}

		for orgId, count := range orgCaseCounts {
			metrics = append(metrics, models.NewOrganizationMetric(CaseCountMetricName,
				utils.Ptr(float64(count)), nil, orgMap[orgId], period.From, period.To),
			)
		}
	}

	return metrics, nil
}
