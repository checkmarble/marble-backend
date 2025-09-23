package cmd

import "errors"

type CompiledConfig struct {
	Version         string
	SegmentWriteKey string
}

type ServerConfig struct {
	batchIngestionMaxSize            int
	caseManagerBucket                string
	ingestionBucketUrl               string
	offloadingBucketUrl              string
	jwtSigningKey                    string
	jwtSigningKeyFile                string
	loggingFormat                    string
	sentryDsn                        string
	transferCheckEnrichmentBucketUrl string
	telemetryExporter                string
	otelSamplingRates                string
	trigramThreshold                 float64
}

func (config ServerConfig) Validate() error {
	if config.trigramThreshold <= 0 || config.trigramThreshold > 1 {
		return errors.New("trigramThreshold must be between 0 and 1")
	}
	return nil
}
