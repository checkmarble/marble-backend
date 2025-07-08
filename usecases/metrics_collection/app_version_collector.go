package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// Implement GlobalCollector interface for stub global collector
type AppVersionCollector struct {
	apiVersion string
}

func NewAppVersionCollector(apiVersion string) GlobalCollector {
	return AppVersionCollector{
		apiVersion: apiVersion,
	}
}

func (c AppVersionCollector) Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	metrics := make([]models.MetricData, 0)

	metrics = append(metrics, models.NewGlobalMetric("app_version", nil, &c.apiVersion, &from, &to,
		models.MetricCollectionFrequencyInstant))

	return metrics, nil
}
