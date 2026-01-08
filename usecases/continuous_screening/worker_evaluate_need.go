package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type evaluateNeedWorkerRepository interface {
	ListContinuousScreeningConfigByObjectType(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		objectType string,
	) ([]models.ContinuousScreeningConfig, error)
}

type evaluateNeedWorkerClientDbRepository interface {
	ListMonitoredObjectsByObjectIds(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectIds []string,
	) ([]models.ContinuousScreeningMonitoredObject, error)
}

type evaluateNeedWorkerTaskEnqueuer interface {
	EnqueueContinuousScreeningDoScreeningTaskMany(
		ctx context.Context,
		tx repositories.Transaction,
		orgId uuid.UUID,
		objectType string,
		monitoringIds []uuid.UUID,
		triggerType models.ContinuousScreeningTriggerType,
	) error
}

// Worker to check if the object needs to be screened
type EvaluateNeedTaskWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningEvaluateNeedArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo         evaluateNeedWorkerRepository
	clientDbRepo evaluateNeedWorkerClientDbRepository
	taskEnqueuer evaluateNeedWorkerTaskEnqueuer
}

func NewEvaluateNeedTaskWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo evaluateNeedWorkerRepository,
	clientDbRepo evaluateNeedWorkerClientDbRepository,
	taskEnqueuer evaluateNeedWorkerTaskEnqueuer,
) *EvaluateNeedTaskWorker {
	return &EvaluateNeedTaskWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		clientDbRepo:       clientDbRepo,
		taskEnqueuer:       taskEnqueuer,
	}
}

func (w *EvaluateNeedTaskWorker) Timeout(job *river.Job[models.ContinuousScreeningEvaluateNeedArgs]) time.Duration {
	return 10 * time.Second
}

// Job to check if the objects need to be screened based on the list of monitored objects
// The screening is done by the DoScreeningWorker called by the task enqueuer at the end of the job
func (w *EvaluateNeedTaskWorker) Work(
	ctx context.Context,
	job *river.Job[models.ContinuousScreeningEvaluateNeedArgs],
) error {
	// Check if the inserted objects are in the continuous screening list
	if len(job.Args.ObjectIds) > 0 {
		exec := w.executorFactory.NewExecutor()
		clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
		if err != nil {
			return err
		}

		// Check if the object type is configured in the continuous screening config
		configs, err := w.repo.ListContinuousScreeningConfigByObjectType(ctx, exec, job.Args.OrgId, job.Args.ObjectType)
		if err != nil {
			return err
		}
		if len(configs) == 0 {
			// No continuous screening config found, no need to enqueue the task
			return nil
		}

		monitoredObjects, err := w.clientDbRepo.ListMonitoredObjectsByObjectIds(
			ctx,
			clientDbExec,
			job.Args.ObjectType,
			job.Args.ObjectIds,
		)
		if err != nil {
			return err
		}

		if len(monitoredObjects) == 0 {
			// No monitored objects found, no need to enqueue the task
			return nil
		}

		monitoringIds := make([]uuid.UUID, len(monitoredObjects))
		for i, monitoredObject := range monitoredObjects {
			monitoringIds[i] = monitoredObject.Id
		}

		return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			return w.taskEnqueuer.EnqueueContinuousScreeningDoScreeningTaskMany(
				ctx,
				tx,
				job.Args.OrgId,
				job.Args.ObjectType,
				monitoringIds,
				models.ContinuousScreeningTriggerTypeObjectUpdated,
			)
		})
	}
	return nil
}
