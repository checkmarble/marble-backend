package infra

import "time"

type OffloadingConfig struct {
	Enabled         bool
	BucketUrl       string
	JobInterval     time.Duration
	OffloadBefore   time.Duration
	BatchSize       int
	SavepointEvery  int
	WritesPerSecond int
}
