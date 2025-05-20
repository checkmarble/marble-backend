package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/utils"
)

const (
	OFFLOADING_MIN_SAVE_POINTS  = 100
	OFFLOADING_MIN_JOB_INTERVAL = 30 * time.Minute
)

type OffloadingConfig struct {
	Enabled         bool
	BucketUrl       string
	JobInterval     time.Duration
	OffloadBefore   time.Duration
	BatchSize       int
	SavepointEvery  int
	WritesPerSecond int
}

func (cfg *OffloadingConfig) ValidateAndFix(ctx context.Context) {
	logger := utils.LoggerFromContext(ctx)

	if cfg.WritesPerSecond < OFFLOADING_MIN_SAVE_POINTS {
		logger.Warn(fmt.Sprintf("OFFLOADING_SAVE_POINTS should be greater than %[1]d, but is %[2]d, setting to %[1]d", OFFLOADING_MIN_SAVE_POINTS, cfg.WritesPerSecond))
		cfg.WritesPerSecond = OFFLOADING_MIN_SAVE_POINTS
	}
	if cfg.JobInterval.Seconds() < OFFLOADING_MIN_JOB_INTERVAL.Seconds() {
		logger.Warn(fmt.Sprintf("OFFLOADING_JOB_INTERVAL should be greater than %[1]s, but is %[2]s, setting to %[1]s", OFFLOADING_MIN_JOB_INTERVAL, cfg.JobInterval))
		cfg.JobInterval = OFFLOADING_MIN_JOB_INTERVAL
	}

}
