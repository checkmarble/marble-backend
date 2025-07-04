package infra

import "time"

type MetricCollectionConfig struct {
	Enabled     bool
	JobInterval time.Duration
}
