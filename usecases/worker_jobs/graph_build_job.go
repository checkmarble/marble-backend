package worker_jobs

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

// A build reads every live record of every table an organization has, so it is deliberately
// infrequent: the graph is a snapshot, not a live view.
const GRAPH_BUILD_DEFAULT_INTERVAL = 24 * time.Hour

func NewGraphBuildPeriodicJob(orgId uuid.UUID, interval time.Duration) *river.PeriodicJob {
	return NewPeriodicJob(
		river.PeriodicInterval(interval),
		func() (river.JobArgs, *river.InsertOpts) {
			return models.GraphBuildArgs{
					OrgId: orgId,
				}, &river.InsertOpts{
					Queue: orgId.String(),
					UniqueOpts: river.UniqueOpts{
						ByQueue:  true,
						ByPeriod: interval,
					},
				}
		},
	)
}

// GraphBuilder rebuilds an organization's adjacency table. It is separate from the worker so the
// build can be driven — and tested — without going through the queue.
type GraphBuilder struct {
	executorFactory         executor_factory.ExecutorFactory
	transactionFactory      executor_factory.TransactionFactory
	dataModelRepository     repositories.DataModelRepository
	graphRelationRepository repositories.GraphRelationRepository
	graphBuilderRepository  repositories.GraphBuilderRepository
}

func NewGraphBuilder(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	dataModelRepository repositories.DataModelRepository,
	graphRelationRepository repositories.GraphRelationRepository,
	graphBuilderRepository repositories.GraphBuilderRepository,
) GraphBuilder {
	return GraphBuilder{
		executorFactory:         executorFactory,
		transactionFactory:      transactionFactory,
		dataModelRepository:     dataModelRepository,
		graphRelationRepository: graphRelationRepository,
		graphBuilderRepository:  graphBuilderRepository,
	}
}

// Build writes a whole new adjacency table and swaps it in once it is complete, so a walk running
// while a build is in flight keeps reading the previous graph rather than a half-filled one.
func (b GraphBuilder) Build(ctx context.Context, organizationId uuid.UUID) error {
	logger := utils.LoggerFromContext(ctx)
	start := time.Now()

	marbleExec := b.executorFactory.NewExecutor()

	dataModel, err := b.dataModelRepository.GetDataModel(ctx, marbleExec, organizationId, false, false)
	if err != nil {
		return errors.Wrap(err, "could not read the data model")
	}
	if len(dataModel.Tables) == 0 {
		logger.DebugContext(ctx, "graph build: organization has no data model, nothing to build",
			"org_id", organizationId)
		return nil
	}

	relations, err := b.graphRelationRepository.ListGraphRelations(ctx, marbleExec, organizationId)
	if err != nil {
		return errors.Wrap(err, "could not read the graph relations")
	}

	// The same derivation the walk uses to decide what it may read, plus object_id everywhere.
	// Sharing it is what keeps the walk from reading a field the table was never told to carry.
	fieldsByType := models.GraphIndexedFields(dataModel, relations)

	clientExec, err := b.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	if err := b.graphBuilderRepository.CreateGraphBuildTable(ctx, clientExec); err != nil {
		return err
	}

	var rows int64
	for _, recordType := range slices.Sorted(maps.Keys(fieldsByType)) {
		written, err := b.graphBuilderRepository.PopulateGraphBuildTable(ctx,
			clientExec, recordType, fieldsByType[recordType])
		if err != nil {
			// Leave nothing behind for the next run to trip over. The live table is untouched.
			b.dropBuildTable(ctx, clientExec)
			return err
		}
		rows += written
	}

	if err := b.graphBuilderRepository.IndexGraphBuildTable(ctx, clientExec); err != nil {
		b.dropBuildTable(ctx, clientExec)
		return err
	}

	err = b.transactionFactory.TransactionInOrgSchema(ctx, organizationId,
		func(tx repositories.Transaction) error {
			return b.graphBuilderRepository.SwapGraphTable(ctx, tx)
		})
	if err != nil {
		b.dropBuildTable(ctx, clientExec)
		return err
	}

	logger.InfoContext(ctx, "graph build completed",
		"org_id", organizationId,
		"record_types", len(fieldsByType),
		"relations", len(relations),
		"rows", rows,
		"duration", time.Since(start).String(),
	)

	return nil
}

// dropBuildTable cleans up after a failed build. The error is logged rather than returned: it
// would mask the failure that led here, and the next run drops the table before creating it.
func (b GraphBuilder) dropBuildTable(ctx context.Context, exec repositories.Executor) {
	if err := b.graphBuilderRepository.DropGraphBuildTable(ctx, exec); err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"graph build: could not drop the build table after a failure", "error", err)
	}
}

type GraphBuildWorker struct {
	river.WorkerDefaults[models.GraphBuildArgs]

	builder         GraphBuilder
	executorFactory executor_factory.ExecutorFactory
	interval        time.Duration
}

func NewGraphBuildWorker(
	builder GraphBuilder,
	executorFactory executor_factory.ExecutorFactory,
	interval time.Duration,
) *GraphBuildWorker {
	return &GraphBuildWorker{
		builder:         builder,
		executorFactory: executorFactory,
		interval:        interval,
	}
}

func (w *GraphBuildWorker) Timeout(job *river.Job[models.GraphBuildArgs]) time.Duration {
	return time.Hour
}

func (w *GraphBuildWorker) Work(ctx context.Context, job *river.Job[models.GraphBuildArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	// Every organization's job is scheduled on the same interval, so without this they would all
	// start their scan at the same moment.
	if err := AddStrideDelay(job, w.interval); err != nil {
		return err
	}

	// A build is long enough that the interval-based uniqueness of the periodic job is not on its
	// own enough to keep two of them off the same organization.
	exec, release, err := w.executorFactory.NewPinnedExecutor(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get pinned executor")
	}
	defer release()

	// Should the worker be killed without closing the connection, this is what eventually gets
	// Postgres to release the lock.
	timeout := w.Timeout(job)
	if _, err := exec.Exec(ctx,
		fmt.Sprintf("SET idle_session_timeout = '%dms'", timeout.Milliseconds())); err != nil {
		return errors.Wrap(err, "failed to set idle_session_timeout")
	}

	lockKey := fmt.Sprintf("graph-build-%s", job.Args.OrgId)
	unlock, acquired, err := repositories.GetTryAdvisoryLock(ctx, exec, lockKey)
	if err != nil {
		return errors.Wrap(err, "failed to acquire advisory lock")
	}
	if !acquired {
		logger.DebugContext(ctx, "graph build: another build is already running for this organization, skipping",
			"org_id", job.Args.OrgId)
		return nil
	}
	defer func() {
		if err := unlock(); err != nil {
			logger.ErrorContext(ctx, "graph build: failed to release advisory lock", "error", err)
		}
	}()

	return w.builder.Build(ctx, job.Args.OrgId)
}
