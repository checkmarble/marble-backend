package scheduled_execution

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
)

const (
	INDEX_DELETION_WORKER_INTERVAL = 30 * time.Minute
	INDEX_DELETION_DRY_RUN         = true
)

var (
	indexRolloverWhitelistPrefixes = []string{
		"uniq_idx",
		"nav_",
	}
)

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
	GetRequiredIndices(ctx context.Context, organizationId string) (toCreate []models.AggregateQueryFamily, err error)
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
	logger := utils.LoggerFromContext(ctx)

	exec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	validIndices, err := w.indexEditor.ListAllValidIndexes(ctx, exec)
	if err != nil {
		return err
	}

	requiredFamilies, err := w.indexRepo.GetRequiredIndices(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	errs := make(map[string]error)
	requiredIndices := make([]models.ConcreteIndex, 0)

Indices:
	// Here, we look for any index that fulfills at least one scenario requirement.
	// Indices that do not fulfill any requirements are obsolete and will be candidates for deletion.
	for _, index := range validIndices {
		for _, req := range requiredFamilies {
			for _, fm := range req.ToIndexFamilies().Slice() {
				if index.Covers(fm) {
					requiredIndices = append(requiredIndices, index)
					continue Indices
				}
			}
		}
	}

	optimalIndices := make([]models.ConcreteIndex, 0)

RemoveSubsumedIndices:
	// Optional: from the set of indices fulfilling requirements, select only the largest one.
	// We do this by comparing indices in a cartesian product and only adding those that are not
	// a subset of another index.
	for _, lhs := range requiredIndices {
		for _, rhs := range requiredIndices {
			if lhs.Name() == rhs.Name() {
				continue
			}

			if lhs.IsSubset(rhs) {
				continue RemoveSubsumedIndices
			}
		}

		optimalIndices = append(optimalIndices, lhs)
	}

IndexDeletion:
	for _, index := range validIndices {
		if !strings.HasPrefix(index.Name(), "idx_") {
			continue
		}
		if strings.HasSuffix(index.Name(), "_pkey") {
			continue
		}
		if slices.Contains(indexRolloverWhitelistPrefixes, index.Name()) {
			continue
		}

		for _, optimalIndex := range optimalIndices {
			if optimalIndex.Equal(index) {
				continue IndexDeletion
			}
		}

		logger.Debug(fmt.Sprintf("index %s.%s.%s is candidate for deletion", exec.DatabaseSchema().Schema, index.TableName, index.Name()),
			"schema", exec.DatabaseSchema().Schema,
			"table", index.TableName,
			"index", index.Name(),
			"dry_run", INDEX_DELETION_DRY_RUN)

		if !INDEX_DELETION_DRY_RUN {
			err := w.indexEditor.DeleteIndex(ctx, exec, index.Name())

			switch err {
			case nil:
				logger.Info(fmt.Sprintf("index %s.%s.%s was deleted successfully", exec.DatabaseSchema().Schema, index.TableName, index.Name()),
					"schema", exec.DatabaseSchema().Schema,
					"table", index.TableName,
					"index", index.Name())
			default:
				errs[index.Name()] = err
			}
		}
	}

	if len(errs) > 0 {
		logger.Error("some indices failed to be deleted",
			"indices", slices.Collect(maps.Keys(errs)),
			"errors", pure_utils.Map(slices.Collect(maps.Values(errs)), func(err error) string {
				return err.Error()
			}))

		return errors.New("could not delete all outdated indices")
	}

	return nil
}
