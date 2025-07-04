package infra

import (
	"net/url"
	"time"

	"github.com/cockroachdb/errors"
)

type MetricCollectionConfig struct {
	Enabled             bool
	JobInterval         time.Duration
	MetricsIngestionUrl string
}

// If metrics collection is enabled, the metrics ingestion url must be set
func (cfg MetricCollectionConfig) Validate() error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.MetricsIngestionUrl == "" {
		return errors.New("metrics ingestion url is not set")
	}

	if _, err := url.ParseRequestURI(cfg.MetricsIngestionUrl); err != nil {
		return errors.Newf("invalid metrics ingestion url: %w", err)
	}

	return nil
}
