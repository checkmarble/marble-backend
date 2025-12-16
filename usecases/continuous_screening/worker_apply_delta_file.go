// TODO: Implement the delta file update worker, create a stub for now to test the workflow
package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type applyDeltaFileWorkerRepository interface {
	GetEnrichedContinuousScreeningUpdateJob(
		ctx context.Context,
		exec repositories.Executor,
		updateId uuid.UUID,
	) (models.EnrichedContinuousScreeningUpdateJob, error)
	UpdateContinuousScreeningUpdateJob(
		ctx context.Context,
		exec repositories.Executor,
		updateId uuid.UUID,
		status models.ContinuousScreeningUpdateJobStatus,
	) error
}

type ApplyDeltaFileWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningApplyDeltaFileArgs]

	executorFactory executor_factory.ExecutorFactory
	repository      applyDeltaFileWorkerRepository
}

func NewApplyDeltaFileWorker(
	executorFactory executor_factory.ExecutorFactory,
	repository applyDeltaFileWorkerRepository,
) *ApplyDeltaFileWorker {
	return &ApplyDeltaFileWorker{
		executorFactory: executorFactory,
		repository:      repository,
	}
}

func (w *ApplyDeltaFileWorker) Timeout(job *river.Job[models.ContinuousScreeningApplyDeltaFileArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *ApplyDeltaFileWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningApplyDeltaFileArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	logger.DebugContext(
		ctx,
		"Starting continuous screening apply delta file update",
		"update_id", job.Args.UpdateId,
		"org_id", job.Args.OrgId,
	)

	updateJob, err := w.repository.GetEnrichedContinuousScreeningUpdateJob(ctx,
		w.executorFactory.NewExecutor(), job.Args.UpdateId)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Enriched continuous screening update job", "update_job", updateJob)

	if updateJob.Status == models.ContinuousScreeningUpdateJobStatusCompleted {
		logger.DebugContext(ctx, "Continuous screening update job already completed, skip processing")
		return nil
	}

	// TODO: Implement the delta file update logic

	err = w.repository.UpdateContinuousScreeningUpdateJob(
		ctx,
		w.executorFactory.NewExecutor(),
		updateJob.Id,
		models.ContinuousScreeningUpdateJobStatusCompleted,
	)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Successfully updated continuous screening update job", "update_job", updateJob)
	return nil
}
