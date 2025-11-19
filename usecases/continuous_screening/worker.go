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

type repository interface {
	GetContinuousScreeningConfigByStableId(
		ctx context.Context,
		exec repositories.Executor,
		stableId uuid.UUID,
	) (models.ContinuousScreeningConfig, error)

	InsertContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		screening models.ScreeningWithMatches,
		config models.ContinuousScreeningConfig,
		objectType string,
		objectId string,
		objectInternalId uuid.UUID,
		triggerType models.ContinuousScreeningTriggerType,
	) (models.ContinuousScreeningWithMatches, error)
}

type clientDbRepository interface {
	GetMonitoredObject(
		ctx context.Context,
		clientExec repositories.Executor,
		objectType string,
		monitoringId uuid.UUID,
	) (models.ContinuousScreeningMonitoredObject, error)
}

type DoScreeningWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningDoScreeningArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo         repository
	clientDbRepo clientDbRepository
	usecase      ContinuousScreeningUsecase
}

func NewDoScreeningWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo repository,
	clientDbRepo clientDbRepository,
	uc ContinuousScreeningUsecase,
) *DoScreeningWorker {
	return &DoScreeningWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		clientDbRepo:       clientDbRepo,
		usecase:            uc,
	}
}

func (w *DoScreeningWorker) Timeout(job *river.Job[models.ContinuousScreeningDoScreeningArgs]) time.Duration {
	return 10 * time.Second
}

func (w *DoScreeningWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningDoScreeningArgs]) error {
	exec := w.executorFactory.NewExecutor()
	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	// Fetch the monitored object from client DB
	monitoredObject, err := w.clientDbRepo.GetMonitoredObject(
		ctx,
		clientDbExec,
		job.Args.ObjectType,
		job.Args.MonitoringId,
	)
	if err != nil {
		return err
	}

	// Fetch the configuration
	config, err := w.repo.GetContinuousScreeningConfigByStableId(ctx, exec, monitoredObject.ConfigStableId)
	if err != nil {
		return err
	}

	// Have the data model table and mapping
	table, mapping, err := w.usecase.GetDataModelTableAndMapping(ctx, exec, config, job.Args.ObjectType)
	if err != nil {
		return err
	}

	// Fetch the ingested Data
	ingestedObject, ingestedObjectInternalId, err :=
		w.usecase.GetIngestedObject(ctx, clientDbExec, table, monitoredObject.ObjectId)
	if err != nil {
		return err
	}

	// Do the screening
	screeningWithMatches, err := w.usecase.DoScreening(ctx, ingestedObject, mapping, config)
	if err != nil {
		return err
	}

	return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// Insert the continuous screening result
		continuousScreeningWithMatches, err := w.repo.InsertContinuousScreening(
			ctx,
			tx,
			screeningWithMatches,
			config,
			job.Args.ObjectType,
			monitoredObject.ObjectId,
			ingestedObjectInternalId,
			models.ContinuousScreeningTriggerType(job.Args.TriggerType),
		)
		if err != nil {
			return err
		}

		// Create the case if needed
		return w.usecase.HandleCaseCreation(
			ctx,
			tx,
			screeningWithMatches,
			config,
			monitoredObject.ObjectId,
			continuousScreeningWithMatches,
		)
	})
}
