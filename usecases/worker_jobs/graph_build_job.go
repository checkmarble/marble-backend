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

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/utils"
)

type featureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId uuid.UUID,
		user *models.UserId,
	) (models.OrganizationFeatureAccess, error)
}

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

type GraphBuilder struct {
	executorFactory         executor_factory.ExecutorFactory
	transactionFactory      executor_factory.TransactionFactory
	featureAccessReader     featureAccessReader
	dataModelRepository     repositories.DataModelRepository
	graphRelationRepository repositories.GraphRelationRepository
	graphBuilderRepository  repositories.GraphBuilderRepository
}

func NewGraphBuilder(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	featureAccessReader feature_access.FeatureAccessReader,
	dataModelRepository repositories.DataModelRepository,
	graphRelationRepository repositories.GraphRelationRepository,
	graphBuilderRepository repositories.GraphBuilderRepository,
) GraphBuilder {
	return GraphBuilder{
		executorFactory:         executorFactory,
		transactionFactory:      transactionFactory,
		featureAccessReader:     featureAccessReader,
		dataModelRepository:     dataModelRepository,
		graphRelationRepository: graphRelationRepository,
		graphBuilderRepository:  graphBuilderRepository,
	}
}

func (w GraphBuilder) Build(ctx context.Context, organizationId uuid.UUID) error {
	fa, err := w.featureAccessReader.GetOrganizationFeatureAccess(ctx, organizationId, nil)
	if err != nil {
		return errors.Wrap(err, "could not check organization feature access from worker")
	}

	if !fa.GraphExploration.IsAllowed() {
		return nil
	}

	logger := utils.LoggerFromContext(ctx)
	start := time.Now()

	exec := w.executorFactory.NewExecutor()

	dataModel, err := w.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, false)
	if err != nil {
		return errors.Wrap(err, "could not read the data model")
	}
	if len(dataModel.Tables) == 0 {
		logger.DebugContext(ctx, "graph build: organization has no data model, nothing to build",
			"org_id", organizationId)
		return nil
	}

	relations, err := w.graphRelationRepository.ListGraphRelations(ctx, exec, organizationId)
	if err != nil {
		return errors.Wrap(err, "could not read the graph relations")
	}

	fieldsByType := models.GraphIndexedFields(dataModel, relations)

	clientExec, err := w.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	if err := w.graphBuilderRepository.CreateGraphBuildTable(ctx, clientExec); err != nil {
		return err
	}

	// Read before the first populate, so that everything the incremental ingestion writer commits
	// from here on is either seen by a populate below or replayed before the swap. Erring early is
	// free: the replay's upsert is idempotent, so an over-inclusive watermark only costs rows that
	// restate what the build already has.
	watermark, err := w.graphBuilderRepository.GraphReplayWatermark(ctx, clientExec)
	if err != nil {
		w.dropBuildTable(ctx, clientExec)
		return err
	}

	var rows int64

	for _, recordType := range slices.Sorted(maps.Keys(fieldsByType)) {
		written, err := w.graphBuilderRepository.PopulateGraphBuildTable(ctx, clientExec, recordType, fieldsByType[recordType])
		if err != nil {
			w.dropBuildTable(ctx, clientExec)
			return err
		}
		rows += written
	}

	// Indexing first: the replay's upsert needs the primary key to have a conflict target.
	if err := w.graphBuilderRepository.IndexGraphBuildTable(ctx, clientExec); err != nil {
		w.dropBuildTable(ctx, clientExec)
		return err
	}

	// Read before the bulk pass below, and deliberately not after: the reconcile has to cover
	// everything that pass might have missed, and a row committing just after its snapshot is stamped
	// around the moment it started. A watermark taken later — after the analyze, say, which on a large
	// table can exceed the margin — would leave such a row in neither pass.
	reconcileWatermark, err := w.graphBuilderRepository.GraphReplayWatermark(ctx, clientExec)
	if err != nil {
		w.dropBuildTable(ctx, clientExec)
		return err
	}

	// The bulk of the catch-up, so the table that goes live is already close to complete. It holds no
	// lock a walk conflicts with, so its duration is nobody's problem.
	replayed, err := w.graphBuilderRepository.ReplayGraphRows(ctx, clientExec, watermark)
	if err != nil {
		w.dropBuildTable(ctx, clientExec)
		return err
	}

	if err := w.graphBuilderRepository.AnalyzeGraphBuildTable(ctx, clientExec); err != nil {
		w.dropBuildTable(ctx, clientExec)
		return err
	}

	err = w.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		return w.graphBuilderRepository.SwapGraphTable(ctx, tx)
	})
	if err != nil {
		w.dropBuildTable(ctx, clientExec)
		return err
	}

	// Past the swap the new table is live, so a failure here is no longer worth failing the build over:
	// the graph is serving, merely missing the tail this would have carried over, and the next build
	// regenerates it from source. The previous generation is left behind for that build to clear.
	written, err := w.reconcile(ctx, organizationId, reconcileWatermark)
	if err != nil {
		logger.ErrorContext(ctx, "graph build: could not reconcile the previous generation, the tail is missing until the next build",
			"org_id", organizationId, "error", err.Error())
	}
	replayed += written

	logger.InfoContext(ctx, "graph build completed",
		"org_id", organizationId,
		"record_types", len(fieldsByType),
		"relations", len(relations),
		"rows", rows,
		"replayed", replayed,
		"duration", time.Since(start).String(),
	)

	return nil
}

func (w GraphBuilder) reconcile(
	ctx context.Context,
	organizationId uuid.UUID,
	since time.Time,
) (int64, error) {
	var replayed int64

	err := w.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		var err error
		replayed, err = w.graphBuilderRepository.ReconcileGraphFromOld(ctx, tx, since)

		return err
	})

	return replayed, err
}

func (w GraphBuilder) dropBuildTable(ctx context.Context, exec repositories.Executor) {
	if err := w.graphBuilderRepository.DropGraphBuildTable(ctx, exec); err != nil {
		utils.LoggerFromContext(ctx).ErrorContext(ctx,
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
	return time.Hour // TODO: this will probably not be enough
}

func (w *GraphBuildWorker) Work(ctx context.Context, job *river.Job[models.GraphBuildArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	if !infra.HasFeatureFlag(infra.GRAPH_EXPLORATION_FEATURE_FLAG, job.Args.OrgId) {
		return nil
	}

	// Prevent herd effect
	if err := AddStrideDelay(job, w.interval); err != nil {
		return err
	}

	exec, release, err := w.executorFactory.NewPinnedExecutor(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get pinned executor")
	}
	defer release()

	timeout := w.Timeout(job)

	if _, err := exec.Exec(ctx,
		fmt.Sprintf("set idle_session_timeout = '%dms'", timeout.Milliseconds())); err != nil {
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
