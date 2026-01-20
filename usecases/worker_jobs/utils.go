package worker_jobs

import (
	"math/rand/v2"
	"time"

	"github.com/riverqueue/river"
	"github.com/tidwall/gjson"
)

func AddStrideDelay[T river.JobArgs](job *river.Job[T], interval time.Duration) error {
	if gjson.GetBytes(job.Metadata, "snoozes").Int() > 0 {
		return nil
	}
	if gjson.GetBytes(job.Metadata, "manual").Bool() {
		return nil
	}

	delay := time.Duration(rand.IntN(int((interval / 2).Seconds()))) * time.Second

	return river.JobSnooze(delay)
}
