package infra

import (
	"time"
)

type MetricCollectionConfig struct {
	Enabled             bool
	JobInterval         time.Duration
	MetricsIngestionURL string
}

// TODO: Before deploying in production, change the default url to the production url
const DEFAULT_METRICS_INGESTION_URL = "http://localhost:8080/metrics"

var PROJECT_ID_TO_URL = map[string]string{
	"marble-prod-1":        "https://api.checkmarble.com/metrics",
	"tokyo-country-381508": "https://api.staging.checkmarble.com/metrics",
}

// If metrics collection is enabled, build the metrics ingestion url from the project id
func (cfg *MetricCollectionConfig) Configure() {
	if !cfg.Enabled {
		return
	}

	// Build the MetricsIngestionURL from the project id
	projectId, _ := GetProjectId()
	cfg.MetricsIngestionURL = DEFAULT_METRICS_INGESTION_URL
	if url, ok := PROJECT_ID_TO_URL[projectId]; ok {
		cfg.MetricsIngestionURL = url
	}
}
