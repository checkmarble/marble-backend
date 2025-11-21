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
	GetContinuousScreeningByObjectId(
		ctx context.Context,
		exec repositories.Executor,
		objectId string,
		objectType string,
		orgId uuid.UUID,
	) (*models.ContinuousScreeningWithMatches, error)
}

type clientDbRepository interface {
	GetMonitoredObject(
		ctx context.Context,
		clientExec repositories.Executor,
		objectType string,
		monitoringId uuid.UUID,
	) (models.ContinuousScreeningMonitoredObject, error)
}

type continuousScreeningUsecase interface {
	GetDataModelTableAndMapping(ctx context.Context, exec repositories.Executor,
		config models.ContinuousScreeningConfig, objectType string,
	) (models.Table, models.ContinuousScreeningDataModelMapping, error)
	GetIngestedObject(ctx context.Context, clientDbExec repositories.Executor, table models.Table,
		objectId string,
	) (models.DataModelObject, uuid.UUID, error)
	DoScreening(ctx context.Context, ingestedObject models.DataModelObject,
		mapping models.ContinuousScreeningDataModelMapping,
		config models.ContinuousScreeningConfig,
	) (models.ScreeningWithMatches, error)
	HandleCaseCreation(ctx context.Context, tx repositories.Transaction,
		config models.ContinuousScreeningConfig, objectId string,
		continuousScreeningWithMatches models.ContinuousScreeningWithMatches) error
}

type DoScreeningWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningDoScreeningArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo         repository
	clientDbRepo clientDbRepository
	usecase      continuousScreeningUsecase
}

func NewDoScreeningWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo repository,
	clientDbRepo clientDbRepository,
	uc continuousScreeningUsecase,
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

// Work executes the continuous screening process for a specific monitored object.
// The flow consists of the following steps:
//  1. Retrieve the monitored object details from the client's database which contains the object ID and the screening configuration ID.
//  2. Fetch the associated continuous screening configuration.
//  3. Determine the data model table and field mapping for the object type for opensanction query.
//  4. Fetch the actual ingested object data from the client's database.
//  5. Perform the screening against the configured watchlist/rules.
//  6. If the trigger is an object update, check if the screening results (matches) have changed compared to the latest in review and attached with a case screening result.
//     If unchanged, case creation is skipped to avoid redundant case creation.
//  7. Persist the screening results and, if applicable (and not skipped), handle case creation within a transaction.
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

	skipCaseCreation := false
	// Only in case of Object updated by the user, check if the screening result is the same as the existing one (if exists)
	if job.Args.TriggerType == models.ContinuousScreeningTriggerTypeObjectUpdated {
		skipCaseCreation, err = w.isScreeningResultUnchanged(
			ctx,
			exec,
			screeningWithMatches,
			monitoredObject.ObjectId,
			job.Args.ObjectType,
			config.OrgId,
		)
		if err != nil {
			return err
		}
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
			job.Args.TriggerType,
		)
		if err != nil {
			return err
		}

		if !skipCaseCreation && screeningWithMatches.Status == models.ScreeningStatusInReview {
			return w.usecase.HandleCaseCreation(
				ctx,
				tx,
				config,
				monitoredObject.ObjectId,
				continuousScreeningWithMatches,
			)
		}
		return nil
	})
}

// Compare matches of the existing and new screening results
// The check is based on OpenSanction entity ID only and we suppose matches are unique and not duplicated
func areScreeningMatchesEqual(
	existingScreeningWithMatches models.ContinuousScreeningWithMatches,
	newScreeningWithMatches models.ScreeningWithMatches,
) bool {
	if len(existingScreeningWithMatches.Matches) != len(newScreeningWithMatches.Matches) {
		return false
	}

	existingMatches := make(
		map[string]bool,
		len(existingScreeningWithMatches.Matches),
	)
	for _, match := range existingScreeningWithMatches.Matches {
		existingMatches[match.OpenSanctionEntityId] = true
	}

	for _, match := range newScreeningWithMatches.Matches {
		if !existingMatches[match.EntityId] {
			return false
		}
	}

	return true
}

// Get the latest in review screening result attached to a case for the object
// and compare it with the new screening result
func (w *DoScreeningWorker) isScreeningResultUnchanged(
	ctx context.Context,
	exec repositories.Executor,
	newScreeningWithMatches models.ScreeningWithMatches,
	objectId string,
	objectType string,
	orgId uuid.UUID,
) (bool, error) {
	existingScreeningWithMatches, err := w.repo.GetContinuousScreeningByObjectId(
		ctx,
		exec,
		objectId,
		objectType,
		orgId,
	)
	if err != nil {
		return false, err
	}

	if existingScreeningWithMatches != nil {
		return areScreeningMatchesEqual(
			*existingScreeningWithMatches,
			newScreeningWithMatches,
		), nil
	}
	return false, nil
}
