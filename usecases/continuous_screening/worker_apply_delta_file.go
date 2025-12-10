// TODO: Implement the delta file update worker, create a stub for now to test the workflow
package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

type ApplyDeltaFileWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningApplyDeltaFileArgs]

	executorFactory executor_factory.ExecutorFactory
}

func NewApplyDeltaFileWorker(
	executorFactory executor_factory.ExecutorFactory,
) *ApplyDeltaFileWorker {
	return &ApplyDeltaFileWorker{
		executorFactory: executorFactory,
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
	return nil
}
