package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type MetricCollectionConfig struct {
	Disabled            bool
	JobInterval         time.Duration
	MetricsIngestionURL string // Build from environment, call Configure() to set it
	FallbackDuration    time.Duration
	JobTimeout          time.Duration // METRICS_COLLECTION_JOB_TIMEOUT, default 10m
	CollectorTimeout    time.Duration // METRICS_COLLECTOR_TIMEOUT, default 2m
}

const (
	METRICS_DEFAULT_JOB_TIMEOUT       = 10 * time.Minute
	METRICS_DEFAULT_COLLECTOR_TIMEOUT = 2 * time.Minute
	METRICS_MIN_COLLECTION_BUDGET     = 30 * time.Second
)

// Time reserved for sending the metrics, outside of the collection budget
const METRICS_SEND_TIMEOUT = 1 * time.Minute

// Time allowed to save the watermarks once the metrics have been sent
const METRICS_SAVE_TIMEOUT = 30 * time.Second

// Time available to the collectors within a job run: the job timeout minus what is reserved for
// sending and saving, so that the whole of Work fits within River's job timeout.
func (cfg MetricCollectionConfig) CollectionBudget() time.Duration {
	return cfg.JobTimeout - METRICS_SEND_TIMEOUT - METRICS_SAVE_TIMEOUT
}

// ValidateAndFix applies defaults and clamps the timeouts, in this order:
//   - JobTimeout and CollectorTimeout default to 10m and 2m when unset or not positive.
//   - JobTimeout is raised to at least SEND + SAVE + METRICS_MIN_COLLECTION_BUDGET (2m), so that the collectors
//     always get some time.
//   - CollectorTimeout is capped at the collection budget (JobTimeout - SEND - SAVE), since a single collector
//     cannot run longer than the whole collection.
//
// JobTimeout may exceed JobInterval: the job is unique among unfinished runs, so a run still in progress makes the
// next one be skipped instead of overlapping it.
func (cfg *MetricCollectionConfig) ValidateAndFix(ctx context.Context) {
	logger := utils.LoggerFromContext(ctx)

	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = METRICS_DEFAULT_JOB_TIMEOUT
	}
	if cfg.CollectorTimeout <= 0 {
		cfg.CollectorTimeout = METRICS_DEFAULT_COLLECTOR_TIMEOUT
	}

	if cfg.CollectionBudget() < METRICS_MIN_COLLECTION_BUDGET {
		minTimeout := METRICS_SEND_TIMEOUT + METRICS_SAVE_TIMEOUT + METRICS_MIN_COLLECTION_BUDGET
		logger.Warn(fmt.Sprintf("METRICS_COLLECTION_JOB_TIMEOUT %[1]s is too short to leave %[2]s for collection, setting to %[3]s", cfg.JobTimeout, METRICS_MIN_COLLECTION_BUDGET, minTimeout))
		cfg.JobTimeout = minTimeout
	}

	if budget := cfg.CollectionBudget(); cfg.CollectorTimeout > budget {
		logger.Warn(fmt.Sprintf("METRICS_COLLECTOR_TIMEOUT should not exceed the collection budget, but is %[1]s, setting to %[2]s", cfg.CollectorTimeout, budget))
		cfg.CollectorTimeout = budget
	}
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
func (cfg *MetricCollectionConfig) Configure(licenseConfig models.LicenseConfiguration) {
	// If the license key is set, metrics collection cannot be disabled
	if licenseConfig.LicenseKey != "" {
		cfg.Disabled = false
	}

	projectId, _ := GetProjectId()
	cfg.MetricsIngestionURL = PROD_METRICS_INGESTION_URL
	if url, ok := PROJECT_ID_TO_URL[projectId]; ok {
		cfg.MetricsIngestionURL = url
	}
}
