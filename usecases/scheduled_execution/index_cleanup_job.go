package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

const INDEX_CLEANUP_WORKER_INTERVAL = time.Hour

func NewIndexCleanupPeriodicJob(orgId string) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(10*time.Second),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.IndexCleanupArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId,
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type IndexCleanupWorker struct {
	river.WorkerDefaults[models.IndexCleanupArgs]

	executorFactory executor_factory.ExecutorFactory
	indexEditor     indexes.IngestedDataIndexesRepository
}

func NewIndexCleanupWorker(
	executor_factory executor_factory.ExecutorFactory,
	indexEditor indexes.IngestedDataIndexesRepository,
) IndexCleanupWorker {
	return IndexCleanupWorker{
		executorFactory: executor_factory,
		indexEditor:     indexEditor,
	}
}

func (w *IndexCleanupWorker) Work(ctx context.Context, job *river.Job[models.IndexCleanupArgs]) error {
	utils.LoggerFromContext(ctx).DebugContext(ctx, "index cleanup", "org", job.Args.OrgId)

	return nil
}
