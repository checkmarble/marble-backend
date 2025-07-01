package metrics_collection

import (
	"context"
	"time"
)

// Implement Collector interface for stub collector
// Example collector that demonstrates time range splitting
type StubCollector struct{}

func NewStubCollector() Collector {
	return StubCollector{}
}

func (c StubCollector) Name() string {
	return "stub"
}

func (c StubCollector) Collect(ctx context.Context, orgId string) ([]MetricData, error) {
	var metrics []MetricData

	// Simple instant metrics
	metrics = append(metrics,
		NewOrganizationMetric("stub_info", "STUB_VALUE", orgId).WithInfo("Example stub metric"),
		NewOrganizationMetric("stub_counter", 42, orgId),
	)

	// Time range metric that gets split by day
	// Example: collecting data from 2 days ago to now
	from := time.Now().AddDate(0, 0, -2) // 2 days ago
	to := time.Now()

	// This will create multiple metrics, one for each day
	dailyMetrics := CreateTimeRangeMetrics(
		"daily_events",
		from,
		to,
		FrequencyDaily,
		false, // not global
		orgId,
		func(periodFrom, periodTo time.Time) any {
			// Calculate value for this specific time period
			// In real implementation, you'd query your database for this period
			hours := periodTo.Sub(periodFrom).Hours()
			return int(hours * 10) // Mock: 10 events per hour
		},
	)

	metrics = append(metrics, dailyMetrics...)

	// Monthly metric example
	monthFrom := time.Now().AddDate(0, -1, 0) // 1 month ago
	monthTo := time.Now()

	monthlyMetrics := CreateTimeRangeMetrics(
		"monthly_revenue",
		monthFrom,
		monthTo,
		FrequencyMonthly,
		false,
		orgId,
		func(periodFrom, periodTo time.Time) any {
			// Mock revenue calculation for the period
			days := periodTo.Sub(periodFrom).Hours() / 24
			return days * 1000.0 // $1000 per day
		},
	)

	metrics = append(metrics, monthlyMetrics...)

	return metrics, nil
}
