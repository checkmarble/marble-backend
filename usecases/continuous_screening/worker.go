package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
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
		status *models.ScreeningStatus,
		inCase bool,
	) (*models.ContinuousScreeningWithMatches, error)
}

type clientDbRepository interface {
	GetMonitoredObject(
		ctx context.Context,
		clientExec repositories.Executor,
		monitoringId uuid.UUID,
	) (models.ContinuousScreeningMonitoredObject, error)

	ListMonitoredObjectsByObjectIds(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectIds []string,
	) ([]models.ContinuousScreeningMonitoredObject, error)
}

type continuousScreeningUsecase interface {
	GetDataModelTableAndMapping(ctx context.Context, exec repositories.Executor,
		config models.ContinuousScreeningConfig, objectType string,
	) (models.Table, models.ContinuousScreeningDataModelMapping, error)
	GetIngestedObject(ctx context.Context, clientDbExec repositories.Executor, table models.Table,
		objectId string,
	) (models.DataModelObject, uuid.UUID, error)
	DoScreening(
		ctx context.Context,
		exec repositories.Executor,
		ingestedObject models.DataModelObject,
		mapping models.ContinuousScreeningDataModelMapping,
		config models.ContinuousScreeningConfig,
		objectType string,
		objectId string,
	) (models.ScreeningWithMatches, error)
	HandleCaseCreation(ctx context.Context, tx repositories.Transaction,
		config models.ContinuousScreeningConfig, objectId string,
		continuousScreeningWithMatches models.ContinuousScreeningWithMatches) error
}

// Worker to do the screening for a specific monitored object
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
//  5. Fetch the latest screening result for the object and check if the existing screening is more recent than the ingested object's valid_from timestamp.
//     If so, skip screening to avoid redundant screening on unchanged data.
//  6. Perform the screening against the configured watchlist/rules.
//  7. If the trigger is an object update, check if the screening results (matches) have changed compared to the latest in review and attached with a case screening result.
//     If unchanged, case creation is skipped to avoid redundant case creation.
//  8. Persist the screening results and, if applicable (and not skipped), handle case creation within a transaction.
func (w *DoScreeningWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningDoScreeningArgs]) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	// Fetch the monitored object from client DB
	monitoredObject, err := w.clientDbRepo.GetMonitoredObject(
		ctx,
		clientDbExec,
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

	// Fetch the latest screening result for the object
	existingScreeningWithMatches, err := w.repo.GetContinuousScreeningByObjectId(
		ctx,
		exec,
		monitoredObject.ObjectId,
		job.Args.ObjectType,
		config.OrgId,
		nil,
		false,
	)
	if err != nil {
		return err
	}

	if existingScreeningWithMatches != nil {
		ingestedObjectValidFrom, ok := ingestedObject.Metadata["valid_from"].(time.Time)
		if !ok {
			logger.WarnContext(ctx, "Continuous Screening - valid_from not found in ingested object metadata, skipping screening")
			return nil
		}
		if existingScreeningWithMatches.CreatedAt.After(ingestedObjectValidFrom) {
			logger.InfoContext(ctx, "Continuous Screening - ingested object valid from is before the latest continuous screening result, skipping screening",
				"object_id", monitoredObject.ObjectId,
				"object_type", job.Args.ObjectType,
				"org_id", config.OrgId,
				"ingested_object_valid_from", ingestedObjectValidFrom,
				"screening_creation_date", existingScreeningWithMatches.CreatedAt)
			return nil
		}
	}

	// Do the screening
	screeningWithMatches, err := w.usecase.DoScreening(
		ctx,
		exec,
		ingestedObject,
		mapping,
		config,
		job.Args.ObjectType,
		monitoredObject.ObjectId,
	)
	if err != nil {
		return err
	}

	skipCaseCreation := false
	// Only in case of Object updated by the user, check if the screening result is the same as the existing one (if exists)
	if job.Args.TriggerType == models.ContinuousScreeningTriggerTypeObjectUpdated {
		// This time, we fetch the latest screening result in review and attached to a case to determine if we can skip case creation
		// In case of same matches, we don't need to create a new case for the same result
		lastScreeningWithMatchesInReviewAndInCase, err := w.repo.GetContinuousScreeningByObjectId(
			ctx,
			exec,
			monitoredObject.ObjectId,
			job.Args.ObjectType,
			config.OrgId,
			utils.Ptr(models.ScreeningStatusInReview),
			true,
		)
		if err != nil {
			return err
		}
		if lastScreeningWithMatchesInReviewAndInCase != nil {
			skipCaseCreation = areScreeningMatchesEqual(
				*lastScreeningWithMatchesInReviewAndInCase,
				screeningWithMatches,
			)
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

// Worker to check if the object needs to be screened
type taskEnqueuer interface {
	EnqueueContinuousScreeningDoScreeningTaskMany(
		ctx context.Context,
		tx repositories.Transaction,
		orgId string,
		objectType string,
		monitoringIds []uuid.UUID,
		triggerType models.ContinuousScreeningTriggerType,
	) error
}

type EvaluateNeedTaskWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningEvaluateNeedArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	clientDbRepo clientDbRepository
	taskEnqueuer taskEnqueuer
}

func NewEvaluateNeedTaskWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	clientDbRepo clientDbRepository,
	taskEnqueuer taskEnqueuer,
) *EvaluateNeedTaskWorker {
	return &EvaluateNeedTaskWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
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
		clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
		if err != nil {
			return err
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
