package worker_jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

const (
	ASYNC_DECISION_CLEANUP_INTERVAL = 1 * time.Hour
	ASYNC_DECISION_CLEANUP_TIMEOUT  = 5 * time.Minute
	ASYNC_DECISION_RETENTION_PERIOD = 30 * 24 * time.Hour // 30 days
	ASYNC_DECISION_CLEANUP_QUEUE    = "async_decision_cleanup"
	ASYNC_DECISION_CLEANUP_BATCH    = 1000
)

func NewAsyncDecisionExecutionCleanupPeriodicJob() *river.PeriodicJob {
	return NewPeriodicJob(
		river.PeriodicInterval(ASYNC_DECISION_CLEANUP_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.AsyncDecisionExecutionCleanupArgs{},
				&river.InsertOpts{
					Queue:    ASYNC_DECISION_CLEANUP_QUEUE,
					Priority: 4, // Low priority
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: ASYNC_DECISION_CLEANUP_INTERVAL,
					},
				}
		},
	)
}

// asyncDecisionCleanupRepository defines the interface for cleanup operations.
type asyncDecisionCleanupRepository interface {
	DeleteOldAsyncDecisionExecutionsBatch(ctx context.Context, exec repositories.Executor,
		olderThan time.Time, limit int) (int64, error)
}

// AsyncDecisionExecutionCleanupWorker handles cleanup of old async decision executions.
type AsyncDecisionExecutionCleanupWorker struct {
	river.WorkerDefaults[models.AsyncDecisionExecutionCleanupArgs]

	repository      asyncDecisionCleanupRepository
	executorFactory executor_factory.ExecutorFactory
	retentionPeriod time.Duration
	batchSize       int
}

// NewAsyncDecisionExecutionCleanupWorker creates a new async decision execution cleanup worker.
func NewAsyncDecisionExecutionCleanupWorker(
	repository asyncDecisionCleanupRepository,
	executorFactory executor_factory.ExecutorFactory,
) *AsyncDecisionExecutionCleanupWorker {
	return &AsyncDecisionExecutionCleanupWorker{
		repository:      repository,
		executorFactory: executorFactory,
		retentionPeriod: ASYNC_DECISION_RETENTION_PERIOD,
		batchSize:       ASYNC_DECISION_CLEANUP_BATCH,
	}
}

func (w *AsyncDecisionExecutionCleanupWorker) Timeout(job *river.Job[models.AsyncDecisionExecutionCleanupArgs]) time.Duration {
	return ASYNC_DECISION_CLEANUP_TIMEOUT
}

// Work cleans up old async decision executions in batches.
func (w *AsyncDecisionExecutionCleanupWorker) Work(ctx context.Context, job *river.Job[models.AsyncDecisionExecutionCleanupArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	cutoff := time.Now().Add(-w.retentionPeriod)

	var totalDeleted int64
	for {
		deleted, err := w.repository.DeleteOldAsyncDecisionExecutionsBatch(ctx, exec, cutoff, w.batchSize)
		if err != nil {
			return errors.Wrap(err, "failed to delete old async decision executions")
		}
		totalDeleted += deleted
		if deleted < int64(w.batchSize) {
			break
		}
	}

	if totalDeleted > 0 {
		logger.InfoContext(ctx, "Async decision execution cleanup completed",
			"deleted_executions", totalDeleted,
			"retention_days", int(w.retentionPeriod.Hours()/24))
	}

	return nil
}
