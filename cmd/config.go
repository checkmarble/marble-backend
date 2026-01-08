package cmd

import (
	"errors"
	"time"
)

type CompiledConfig struct {
	Version         string
	SegmentWriteKey string
}

type ServerConfig struct {
	batchIngestionMaxSize            int
	caseManagerBucket                string
	ingestionBucketUrl               string
	offloadingBucketUrl              string
	analyticsBucketUrl               string
	jwtSigningKey                    string
	jwtSigningKeyFile                string
	sentryDsn                        string
	transferCheckEnrichmentBucketUrl string
	telemetryExporter                string
	otelSamplingRates                string
	similarityThreshold              float64
	enableTracing                           bool
	continuousScreeningEntitiesBucketUrl    string
}

func (config ServerConfig) Validate() error {
	if config.similarityThreshold < 0 || config.similarityThreshold > 1 {
		return errors.New("similarityThreshold must be between 0 and 1")
	}
	return nil
}

type WorkerConfig struct {
	appName                     string
	env                         string
	failedWebhooksRetryPageSize int
	ingestionBucketUrl          string
	analyticsBucket             string
	loggingFormat               string
	sentryDsn                   string
	cloudRunProbePort           string
	caseReviewTimeout           time.Duration
	caseManagerBucket           string
	telemetryExporter           string
	otelSamplingRates           string
	enablePrometheus            bool
	enableTracing               bool
	datasetDeltafileBucketUrl   string
	ScanDatasetUpdatesInterval  time.Duration
	CreateFullDatasetInterval                time.Duration
	continuousScreeningEntitiesBucketUrl    string
}
