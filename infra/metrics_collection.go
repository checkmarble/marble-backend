package infra

import (
	"time"
)

type MetricCollectionConfig struct {
	Disabled            bool
	JobInterval         time.Duration
	MetricsIngestionURL string
	FallbackDuration    time.Duration
}

const (
	PROD_METRICS_INGESTION_URL    = "https://api.checkmarble.com/metrics"
	STAGING_METRICS_INGESTION_URL = "https://api.staging.checkmarble.com/metrics"
)

var PROJECT_ID_TO_URL = map[string]string{
	"marble-prod-1":        PROD_METRICS_INGESTION_URL,
	"tokyo-country-381508": STAGING_METRICS_INGESTION_URL,
}

// If metrics collection is enabled, build the metrics ingestion url from the project id
func (cfg *MetricCollectionConfig) Configure() {
	if cfg.Disabled {
		return
	}

	if cfg.MetricsIngestionURL == "" {
		// Build the MetricsIngestionURL from the project id
		// Use the production url by default
		projectId, _ := GetProjectId()
		cfg.MetricsIngestionURL = PROD_METRICS_INGESTION_URL
		if url, ok := PROJECT_ID_TO_URL[projectId]; ok {
			cfg.MetricsIngestionURL = url
		}
	}
}
