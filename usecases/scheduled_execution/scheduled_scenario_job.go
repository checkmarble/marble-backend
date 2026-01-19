package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	SCHEDULED_SCENARIO_INTERVAL = 1 * time.Minute
	SCHEDULED_SCENARIO_TIMEOUT  = 3 * time.Hour
)

func NewScheduledScenarioPeriodicJob(orgId uuid.UUID) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(SCHEDULED_SCENARIO_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.ScheduledScenarioArgs{OrgId: orgId},
				&river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: SCHEDULED_SCENARIO_INTERVAL,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type ScheduledScenarioWorker struct {
	river.WorkerDefaults[models.ScheduledScenarioArgs]

	runScheduledExecution *RunScheduledExecution
}

func NewScheduledScenarioWorker(
	runScheduledExecution *RunScheduledExecution,
) *ScheduledScenarioWorker {
	return &ScheduledScenarioWorker{
		runScheduledExecution: runScheduledExecution,
	}
}

func (w *ScheduledScenarioWorker) Timeout(job *river.Job[models.ScheduledScenarioArgs]) time.Duration {
	return SCHEDULED_SCENARIO_TIMEOUT
}

func (w *ScheduledScenarioWorker) Work(ctx context.Context, job *river.Job[models.ScheduledScenarioArgs]) error {
	return w.runScheduledExecution.ExecuteScheduledScenariosForOrg(ctx, job.Args.OrgId)
}
