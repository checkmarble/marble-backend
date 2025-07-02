package metrics_collection

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

// Implement Collector interface for stub organization collector
type StubOrganizationCollector struct{}

func NewStubOrganizationCollector() Collector {
	return StubOrganizationCollector{}
}

func (c StubOrganizationCollector) Name() string {
	return "stub_organization"
}

func (c StubOrganizationCollector) Collect(ctx context.Context, orgId string) ([]models.MetricData, error) {
	var metrics []models.MetricData

	// Simple instant metrics
	metrics = append(metrics,
		models.NewOrganizationMetric("stub_info", "STUB_VALUE", orgId, nil, nil),
		models.NewOrganizationMetric("stub_counter", 42, orgId, nil, nil),
	)

	return metrics, nil
}

// Implement GlobalCollector interface for stub global collector
type StubGlobalCollector struct{}

func NewStubGlobalCollector() GlobalCollector {
	return StubGlobalCollector{}
}

func (c StubGlobalCollector) Name() string {
	return "stub_global"
}

func (c StubGlobalCollector) Collect(ctx context.Context) ([]models.MetricData, error) {
	return []models.MetricData{
		models.NewGlobalMetric("stub_global", "STUB_VALUE", nil, nil),
	}, nil
}
