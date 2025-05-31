package scheduled_execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/riverqueue/river"
)

const INDEX_DELETION_WORKER_INTERVAL = 5 * time.Second

func NewIndexDeletionPeriodicJob(orgId string) *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(INDEX_DELETION_WORKER_INTERVAL),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.IndexDeletionArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId,
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: INDEX_DELETION_WORKER_INTERVAL,
					},
				}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

type indexDeletionIndexEditor interface {
	GetRequiredIndices(ctx context.Context, organizationId string) (toCreate []models.ConcreteIndex, err error)
}

type IndexDeletionWorker struct {
	river.WorkerDefaults[models.IndexDeletionArgs]

	executorFactory executor_factory.ExecutorFactory
	indexEditor     indexes.IngestedDataIndexesRepository
	indexRepo       indexDeletionIndexEditor
}

func NewIndexDeletionWorker(
	executor_factory executor_factory.ExecutorFactory,
	indexEditor indexes.IngestedDataIndexesRepository,
	indexRepo indexDeletionIndexEditor,
) IndexDeletionWorker {
	return IndexDeletionWorker{
		executorFactory: executor_factory,
		indexEditor:     indexEditor,
		indexRepo:       indexRepo,
	}
}

func (w *IndexDeletionWorker) Work(ctx context.Context, job *river.Job[models.IndexDeletionArgs]) error {
	exec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	validIndices, err := w.indexEditor.ListAllValidIndexes(ctx, exec)
	if err != nil {
		return err
	}

	// TODO: also include required indices for future iterations, not yet live
	requiredIndices, err := w.indexRepo.GetRequiredIndices(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

IndexLoop:
	for _, index := range validIndices {
		// We omit fixed indices that are not created to help aggregate queries.
		if strings.HasSuffix(index.Name(), "_pkey") {
			continue
		}
		if strings.HasPrefix(index.Name(), "uniq_idx_") {
			continue
		}
		if strings.HasPrefix(index.Name(), "nav_") {
			continue
		}

		for _, req := range requiredIndices {
			if req.Equal(index) {
				continue IndexLoop
			}
		}

		fmt.Printf("TO BE DELETED: %#v\n", index)
	}

	return nil
}
