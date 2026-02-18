package worker_jobs

import (
	"github.com/riverqueue/river"
)

// Helper function wrapping NewPeriodicJob so we don't forget to set RunOnStart to true, which is what we want for all our periodic
// jobs so we don't have to wait for AT LEAST one full interval before the first run (even if they are already due).
func NewPeriodicJob(
	scheduleFunc river.PeriodicSchedule,
	constructorFunc river.PeriodicJobConstructor,
) *river.PeriodicJob {
	return river.NewPeriodicJob(scheduleFunc, constructorFunc, &river.PeriodicJobOpts{
		RunOnStart: true,
	})
}
