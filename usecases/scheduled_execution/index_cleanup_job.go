package scheduled_execution

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/hashicorp/go-set/v2"
	"github.com/riverqueue/river"
)

const INDEX_CLEANUP_WORKER_INTERVAL = 10 * time.Second

func NewIndexCleanupPeriodicJob(orgId string) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(INDEX_CLEANUP_WORKER_INTERVAL),
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
	logger := utils.LoggerFromContext(ctx)

	db, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	invalidIndices, err := w.indexEditor.ListInvalidIndices(ctx, db)
	if err != nil {
		return err
	}
	indicesPendingCreation, err := w.indexEditor.ListIndicesPendingCreation(ctx, db)
	if err != nil {
		return err
	}

	deletedIndices := make([]string, 0, len(invalidIndices))

	for _, index := range set.From(invalidIndices).Difference(set.From(indicesPendingCreation)).Slice() {
		logger.DebugContext(ctx, "deleting invalid index", "org", job.Args.OrgId, "index", index)

		if err = w.indexEditor.DeleteInvalidIndex(ctx, db, index); err != nil {
			return err
		}

		deletedIndices = append(deletedIndices, index)
	}

	if len(deletedIndices) > 0 {
		logger.DebugContext(ctx, "deleted invalid indices", "count", len(deletedIndices), "indices", deletedIndices)
	}

	return nil
}
