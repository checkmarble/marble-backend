package infra

import (
	"time"
)

type MetricCollectionConfig struct {
	Disabled            bool
	JobInterval         time.Duration
	MetricsIngestionURL string // Build from environment, call Configure() to set it
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

// Build the metrics ingestion url from the Project ID, use the production url by default
func (cfg *MetricCollectionConfig) Configure() {
	projectId, _ := GetProjectId()
	cfg.MetricsIngestionURL = PROD_METRICS_INGESTION_URL
	if url, ok := PROJECT_ID_TO_URL[projectId]; ok {
		cfg.MetricsIngestionURL = url
	}
}
