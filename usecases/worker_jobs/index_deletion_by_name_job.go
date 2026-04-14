package worker_jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/riverqueue/river"
)

type IndexDeletionByNameWorker struct {
	river.WorkerDefaults[models.IndexDeletionByNameArgs]

	executorFactory executor_factory.ExecutorFactory
	indexEditor     indexes.IngestedDataIndexesRepository
}

func NewIndexDeletionByNameWorker(
	executorFactory executor_factory.ExecutorFactory,
	indexEditor indexes.IngestedDataIndexesRepository,
) *IndexDeletionByNameWorker {
	return &IndexDeletionByNameWorker{
		executorFactory: executorFactory,
		indexEditor:     indexEditor,
	}
}

func (w *IndexDeletionByNameWorker) Timeout(job *river.Job[models.IndexDeletionByNameArgs]) time.Duration {
	return time.Minute
}

func (w *IndexDeletionByNameWorker) Work(ctx context.Context, job *river.Job[models.IndexDeletionByNameArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	exec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	for _, indexName := range job.Args.IndexNames {
		if err := w.indexEditor.DeleteIndex(ctx, exec, indexName); err != nil {
			return fmt.Errorf("failed to delete index %s: %w", indexName, err)
		}
		logger.InfoContext(ctx, "deleted index", "index_name", indexName, "org_id", job.Args.OrgId)
	}

	return nil
}
