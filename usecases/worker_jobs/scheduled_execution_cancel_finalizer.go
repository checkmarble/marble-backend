package worker_jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/checkmarble/marble-backend/models"
)

const scheduledExecCancelFinalizerSnoozeMargin = 1 * time.Second

// ScheduledExecutionCancelFinalizerWorker marks a cancelling scheduled execution as cancelled once
// its cancel deadline has passed. In the normal case, the worker owning the execution acknowledges
// the cancellation well before that and this job is a no-op. It guarantees that a cancellation never
// blocks the scenario forever if that worker died.
type ScheduledExecutionCancelFinalizerWorker struct {
	river.WorkerDefaults[models.ScheduledExecutionCancelFinalizerArgs]

	runScheduledExecution *RunScheduledExecution
}

func NewScheduledExecutionCancelFinalizerWorker(runScheduledExecution *RunScheduledExecution) *ScheduledExecutionCancelFinalizerWorker {
	return &ScheduledExecutionCancelFinalizerWorker{runScheduledExecution: runScheduledExecution}
}

func (w *ScheduledExecutionCancelFinalizerWorker) Timeout(job *river.Job[models.ScheduledExecutionCancelFinalizerArgs]) time.Duration {
	return 10 * time.Second
}

func (w *ScheduledExecutionCancelFinalizerWorker) Work(ctx context.Context, job *river.Job[models.ScheduledExecutionCancelFinalizerArgs]) error {
	repository := w.runScheduledExecution.repository
	exec := w.runScheduledExecution.executorFactory.NewExecutor()

	execution, err := repository.GetScheduledExecution(ctx, exec, job.Args.ScheduledExecutionId)
	if err != nil {
		return err
	}

	// Reads report an execution whose cancel deadline has passed as cancelled, so still being
	// cancelling means the deadline is not reached yet. This can happen because River schedules
	// jobs against the database clock, which may be ahead of the application clock the deadline
	// is measured with. Wait until it is reached.
	if execution.Status == models.ScheduledExecutionCancelling && execution.CancelDeadline != nil {
		return river.JobSnooze(time.Until(*execution.CancelDeadline) + scheduledExecCancelFinalizerSnoozeMargin)
	}

	// No-op unless the stored row is still cancelling, i.e. the worker never acknowledged it. Its
	// deadline is compared with the same clock as above, so it is expired for this update too.
	return repository.FinalizeExpiredCancellations(ctx, exec, models.FinalizeExpiredCancellationsFilters{ScheduledExecutionId: job.Args.ScheduledExecutionId})
}
