package scheduled_execution

import (
	"context"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type IndexCreationWorker struct {
	river.WorkerDefaults[models.IndexCreationArgs]

	executorFactory executor_factory.ExecutorFactory
	indexEditor     indexes.IngestedDataIndexesRepository
}

func NewIndexCreationWorker(
	executor_factory executor_factory.ExecutorFactory,
	indexEditor indexes.IngestedDataIndexesRepository,
) IndexCreationWorker {
	return IndexCreationWorker{
		executorFactory: executor_factory,
		indexEditor:     indexEditor,
	}
}

func (w *IndexCreationWorker) Work(ctx context.Context, job *river.Job[models.IndexCreationArgs]) error {
	client := river.ClientFromContext[pgx.Tx](ctx)

	utils.LoggerFromContext(ctx).DebugContext(ctx, "worker: creating indices", "indices", job.Args.Indices)

	db, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	if err := w.indexEditor.CreateIndexesAsync(ctx, db, job.Args.Indices); err != nil {
		return err
	}

	// TODO: there is a race condition where this runs before the index creation starts, detecting them as failed.
	_, err = client.Insert(
		ctx,
		models.IndexCreationStatusArgs{
			OrgId:   job.Args.OrgId,
			Indices: job.Args.Indices,
		},
		&river.InsertOpts{
			Priority:    1,
			ScheduledAt: time.Now(),
			Queue:       job.Args.OrgId,
		},
	)

	return err
}

type IndexCreationStatusWorker struct {
	river.WorkerDefaults[models.IndexCreationStatusArgs]

	executorFactory executor_factory.ExecutorFactory
	indexEditor     indexes.IngestedDataIndexesRepository
}

func NewIndexCreationStatusWorker(executor_factory executor_factory.ExecutorFactory,
	indexEditor indexes.IngestedDataIndexesRepository,
) IndexCreationStatusWorker {
	return IndexCreationStatusWorker{
		executorFactory: executor_factory,
		indexEditor:     indexEditor,
	}
}

func (w *IndexCreationStatusWorker) Work(ctx context.Context, job *river.Job[models.IndexCreationStatusArgs]) error {
	db, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	pending, err := w.indexEditor.ListIndicesPendingCreation(ctx, db)
	if err != nil {
		return err
	}

	// If we still have pending indices, we are certain the creation is still underway
	if len(pending) > 0 {
		utils.LoggerFromContext(ctx).DebugContext(ctx,
			"worker: index creation still ongoing", "indices", mapIndexNames(job.Args.Indices))

		return river.JobSnooze(1 * time.Second)
	}

	validIndices, err := w.indexEditor.ListAllValidIndexes(ctx, db)
	if err != nil {
		return err
	}

	doneIndices := make([]models.ConcreteIndex, 0, len(job.Args.Indices))

	// Compare the list of finished indices with the list that was supposed to be created,
	// if we find all of them, it means the process successfully finished.
	for _, index := range validIndices {
		if slices.ContainsFunc(job.Args.Indices, func(i models.ConcreteIndex) bool {
			return i.TableName == index.TableName && i.IndexName == index.IndexName
		}) {
			doneIndices = append(doneIndices, index)
		}
	}

	// If there are less finalized indices than were supposed to be created (and no ongoing
	// creation), it means an error occured while creating the indices.
	if len(doneIndices) < len(job.Args.Indices) {
		leftIndices := make([]models.ConcreteIndex, 0, len(job.Args.Indices)-len(doneIndices))

		for _, index := range job.Args.Indices {
			if !slices.ContainsFunc(doneIndices, func(i models.ConcreteIndex) bool {
				return i.TableName == index.TableName && i.IndexName == index.IndexName
			}) {
				leftIndices = append(leftIndices, index)
			}
		}

		_, err := river.ClientFromContext[pgx.Tx](ctx).Insert(
			ctx,
			models.IndexCreationArgs{
				OrgId:   job.Args.OrgId,
				Indices: leftIndices,
			},
			&river.InsertOpts{
				Queue:       job.Args.OrgId,
				ScheduledAt: time.Now().Add(time.Minute),
			},
		)

		utils.LoggerFromContext(ctx).DebugContext(ctx,
			"worker: some index creation failed, resubmitting", "indices", mapIndexNames(leftIndices))

		return err
	}

	// Otherwise, we are done.
	utils.LoggerFromContext(ctx).DebugContext(ctx,
		"worker: finished creating indices", "indices", mapIndexNames(job.Args.Indices))

	return nil
}

func mapIndexNames(indices []models.ConcreteIndex) []string {
	return pure_utils.Map(
		indices, func(i models.ConcreteIndex) string {
			return i.IndexName
		})
}
