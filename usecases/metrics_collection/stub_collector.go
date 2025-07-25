package metrics_collection

// NOTE: This is a stub collector for testing purposes and give an example of how to implement a collector
// A collector should be implemented for each metric type and frequency, collector is responsible of its own frequency
// Maybe we can configure the collector via config file or something else? A collector should not be changed frequently

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

// Implement Collector interface for stub organization collector
type StubOrganizationCollector struct{}

func NewStubOrganizationCollector() Collector {
	return StubOrganizationCollector{}
}

func (c StubOrganizationCollector) Collect(ctx context.Context, orgs []models.Organization,
	from time.Time, to time.Time,
) ([]models.MetricData, error) {
	var metrics []models.MetricData

	// Simple instant metrics
	for _, org := range orgs {
		metrics = append(metrics,
			models.NewOrganizationMetric("stub_info", nil, utils.Ptr("STUB_VALUE"), org.PublicId, from, to),
			models.NewOrganizationMetric("stub_counter", utils.Ptr(float64(42)), nil, org.PublicId, from, to),
		)
	}

	return metrics, nil
}

// Implement GlobalCollector interface for stub global collector
type StubGlobalCollector struct{}

func NewStubGlobalCollector() GlobalCollector {
	return StubGlobalCollector{}
}

func (c StubGlobalCollector) Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	periods, err := pure_utils.SplitTimeRangeByFrequency(from, to, pure_utils.FrequencyMonthly)
	if err != nil {
		return nil, err
	}
	metrics := make([]models.MetricData, len(periods))
	for i, period := range periods {
		metrics[i] = models.NewGlobalMetric("stub_global", nil,
			utils.Ptr(fmt.Sprintf("STUB_VALUE_%d", i)), period.From, period.To)
	}
	return metrics, nil
}
