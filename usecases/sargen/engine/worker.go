package engine

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

// JobStore is the part of the job repository the worker uses: it records the
// River attempt and marks the job failed on the last attempt.
type JobStore interface {
	SetSarGenerationJobStatus(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		status models.SarJobStatus, errMsg string) (bool, error)
	SetSarGenerationJobAttempt(ctx context.Context, exec repositories.Executor, id uuid.UUID, attempt int) error
}

// JobRunner is satisfied by *Orchestrator (a fake in tests).
type JobRunner interface {
	RunJob(ctx context.Context, jobId uuid.UUID) error
}

// Worker is the River worker of models.SarGenerationArgs.
type Worker struct {
	river.WorkerDefaults[models.SarGenerationArgs]

	runner          JobRunner
	jobs            JobStore
	executorFactory ExecutorFactory
	timeout         time.Duration
}

func NewWorker(runner JobRunner, jobs JobStore, ef ExecutorFactory, timeout time.Duration) *Worker {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Worker{runner: runner, jobs: jobs, executorFactory: ef, timeout: timeout}
}

// Timeout is SAR_GENERATION_TIMEOUT; an overrun makes River retry the job,
// which resumes the unfinished tasks.
func (w *Worker) Timeout(*river.Job[models.SarGenerationArgs]) time.Duration { return w.timeout }

// Work runs the job. The River attempt is stored on the job first. A shutdown
// (context canceled, not a timeout) snoozes without consuming an attempt; a
// job or declaration that no longer exists cancels the River job.
// models.LLMRateLimitedError -> river.JobSnooze(RateLimitSnooze)
// (no attempt counted). Any other error is returned for River to retry; when
// job.Attempt >= job.MaxAttempts the job is first set failed (with the error
// message) so the failure is guaranteed to be visible.
func (w *Worker) Work(ctx context.Context, job *river.Job[models.SarGenerationArgs]) error {
	if serr := w.jobs.SetSarGenerationJobAttempt(ctx, w.executorFactory.NewExecutor(), job.Args.JobId, job.Attempt); serr != nil {
		// Informational: never worth failing the job.
		utils.LoggerFromContext(ctx).WarnContext(ctx, "sar generation: cannot store job attempt",
			"job_id", job.Args.JobId, "error", serr.Error())
	}
	err := w.runner.RunJob(ctx, job.Args.JobId)
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		// Shutdown (a timeout would be DeadlineExceeded): the tasks stay
		// resumable, so give the job back without consuming an attempt.
		return river.JobSnooze(0)
	}
	if errors.Is(err, ErrJobGone) {
		return river.JobCancel(err)
	}
	if errors.Is(err, ErrJobNotRunning) {
		// The job was failed or finished elsewhere; a resume enqueues a new run.
		return river.JobCancel(err)
	}
	if errors.Is(err, ErrPromptMissing) || errors.Is(err, ErrLLMUnavailable) || errors.Is(err, ErrRateLimitedTooLong) ||
		errors.Is(err, ErrTaskLimitReached) {
		// The job is already marked failed; retrying cannot help.
		return river.JobCancel(err)
	}
	if errors.Is(err, ErrTasksRunningElsewhere) {
		// Not a failure: wait for the other worker (or the stale delay).
		return river.JobSnooze(RunningElsewhereSnooze)
	}
	if errors.Is(err, models.LLMRateLimitedError) {
		// Snooze does not count an attempt, even on the last one.
		return river.JobSnooze(RateLimitSnooze)
	}
	if job.Attempt >= job.MaxAttempts {
		// Last attempt: make the failure visible before River gives up.
		// Detached: the job context may be done (timeout, shutdown).
		dctx, cancel := detached(ctx)
		defer cancel()
		if _, serr := w.jobs.SetSarGenerationJobStatus(dctx, w.executorFactory.NewExecutor(), job.Args.JobId,
			models.SarJobStatusFailed, PublicError(err)); serr != nil {
			return errors.Wrap(err, "cannot mark job failed: "+serr.Error())
		}
	}
	return err
}
