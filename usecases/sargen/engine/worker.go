package engine

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// JobStatusSetter marks the job failed on the last attempt.
type JobStatusSetter interface {
	SetSarGenerationJobStatus(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		status models.SarJobStatus, errMsg string) error
}

// JobRunner is satisfied by *Orchestrator (a fake in tests).
type JobRunner interface {
	RunJob(ctx context.Context, jobId uuid.UUID) error
}

// Worker is the River worker of models.SarGenerationArgs.
type Worker struct {
	river.WorkerDefaults[models.SarGenerationArgs]

	runner          JobRunner
	jobs            JobStatusSetter
	executorFactory ExecutorFactory
	timeout         time.Duration
}

func NewWorker(runner JobRunner, jobs JobStatusSetter, ef ExecutorFactory, timeout time.Duration) *Worker {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Worker{runner: runner, jobs: jobs, executorFactory: ef, timeout: timeout}
}

// Timeout is SAR_GENERATION_TIMEOUT; an overrun makes River retry the job,
// which resumes the unfinished tasks.
func (w *Worker) Timeout(*river.Job[models.SarGenerationArgs]) time.Duration { return w.timeout }

// Work runs the job. models.LLMRateLimitedError -> river.JobSnooze(RateLimitSnooze)
// (no attempt counted). Any other error is returned for River to retry; when
// job.Attempt >= job.MaxAttempts the job is first set failed (with the error
// message) so the failure is guaranteed to be visible.
func (w *Worker) Work(ctx context.Context, job *river.Job[models.SarGenerationArgs]) error {
	err := w.runner.RunJob(ctx, job.Args.JobId)
	if err == nil {
		return nil
	}
	if errors.Is(err, models.LLMRateLimitedError) {
		// Snooze does not count an attempt, even on the last one.
		return river.JobSnooze(RateLimitSnooze)
	}
	if job.Attempt >= job.MaxAttempts {
		// Last attempt: make the failure visible before River gives up.
		if serr := w.jobs.SetSarGenerationJobStatus(ctx, w.executorFactory.NewExecutor(), job.Args.JobId,
			models.SarJobStatusFailed, err.Error()); serr != nil {
			return errors.Wrap(err, "cannot mark job failed: "+serr.Error())
		}
	}
	return err
}
