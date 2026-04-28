package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type ensureDeltaTrackWorkerRepository interface {
	GetLatestContinuousScreeningDeltaTrack(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		objectType, objectId string,
	) (*models.ContinuousScreeningDeltaTrack, error)
	CreateContinuousScreeningDeltaTrack(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningDeltaTrack,
	) error
}

type ensureDeltaTrackWorkerClientDbRepository interface {
	GetMonitoredObject(
		ctx context.Context,
		exec repositories.Executor,
		id uuid.UUID,
	) (models.ContinuousScreeningMonitoredObject, error)
}

// EnsureDeltaTrackWorker is the safety net for RegisterObjectWorker. It runs ~5 min
// after a registration and creates the Add delta track if the original cross-DB sequence
// (client-DB monitored_object insert + marble-DB delta track insert) committed the first half
// but failed the second half. The marble-DB read of the latest delta track and the recovery
// insert run in a single transaction so a concurrent writer can't slip in between.
type EnsureDeltaTrackWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningEnsureDeltaTrackArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo         ensureDeltaTrackWorkerRepository
	clientDbRepo ensureDeltaTrackWorkerClientDbRepository
}

func NewEnsureDeltaTrackWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo ensureDeltaTrackWorkerRepository,
	clientDbRepo ensureDeltaTrackWorkerClientDbRepository,
) *EnsureDeltaTrackWorker {
	return &EnsureDeltaTrackWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		clientDbRepo:       clientDbRepo,
	}
}

func (w *EnsureDeltaTrackWorker) Timeout(_ *river.Job[models.ContinuousScreeningEnsureDeltaTrackArgs]) time.Duration {
	return time.Minute
}

func (w *EnsureDeltaTrackWorker) Work(
	ctx context.Context,
	job *river.Job[models.ContinuousScreeningEnsureDeltaTrackArgs],
) error {
	logger := utils.LoggerFromContext(ctx)

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	monitoredObject, err := w.clientDbRepo.GetMonitoredObject(ctx, clientDbExec, job.Args.MonitoredObjectId)
	if errors.Is(err, models.NotFoundError) {
		// The monitored_object row referenced by args is absent. Either the client-DB tx that
		// scheduled us rolled back after the enqueue committed, or the object was un-monitored
		// after the original registration. Nothing to repair.
		logger.DebugContext(ctx, "Continuous Screening - verify: monitored object not found, nothing to do",
			"monitored_object_id", job.Args.MonitoredObjectId)
		return nil
	}
	if err != nil {
		return err
	}

	return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		latest, err := w.repo.GetLatestContinuousScreeningDeltaTrack(
			ctx, tx, job.Args.OrgId, job.Args.ObjectType, job.Args.ObjectId,
		)
		if err != nil {
			return err
		}

		// A delta track at least as recent as our specific registration row means either the
		// original Add committed, or a subsequent Update/Delete has advanced state for this
		// entity. In both cases the indexer is already informed — don't override.
		if latest != nil && !latest.CreatedAt.Before(monitoredObject.CreatedAt) {
			return nil
		}

		logger.DebugContext(ctx, "Continuous Screening - verify: delta track missing, recreating",
			"org_id", job.Args.OrgId,
			"object_type", job.Args.ObjectType,
			"object_id", job.Args.ObjectId,
			"operation", job.Args.Operation.String(),
		)
		return w.repo.CreateContinuousScreeningDeltaTrack(ctx, tx, models.CreateContinuousScreeningDeltaTrack{
			OrgId:            job.Args.OrgId,
			ObjectType:       job.Args.ObjectType,
			ObjectId:         job.Args.ObjectId,
			ObjectInternalId: &job.Args.ObjectInternalId,
			EntityId:         job.Args.EntityId,
			Operation:        job.Args.Operation,
		})
	})
}
