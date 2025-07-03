package metrics_collection

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

// Implement GlobalCollector interface for stub global collector
type LicenseKeyCollector struct{}

func NewLicenseKeyCollector() GlobalCollector {
	return LicenseKeyCollector{}
}

func (c LicenseKeyCollector) Collect(ctx context.Context, _ time.Time, _ time.Time) ([]models.MetricData, error) {
	metrics := make([]models.MetricData, 0)

	// Think it is not the best way to get the license key
	licenseKey := utils.GetEnv("LICENSE_KEY", "NO_LICENSE_KEY")

	metrics = append(metrics, models.NewGlobalMetric("license_key", licenseKey, nil, nil,
		models.MetricCollectionFrequencyInstant))

	return metrics, nil
}
