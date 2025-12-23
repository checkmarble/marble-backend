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

type doScreeningWorkerRepository interface {
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

type doScreeningWorkerClientDbRepository interface {
	GetMonitoredObject(
		ctx context.Context,
		clientExec repositories.Executor,
		monitoringId uuid.UUID,
	) (models.ContinuousScreeningMonitoredObject, error)
}

type doScreeningWorkerCSUsecase interface {
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
	HandleCaseCreation(
		ctx context.Context,
		tx repositories.Transaction,
		config models.ContinuousScreeningConfig,
		objectId string,
		continuousScreeningWithMatches models.ContinuousScreeningWithMatches,
	) (models.Case, error)
}

// Worker to do the screening for a specific monitored object
type DoScreeningWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningDoScreeningArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo         doScreeningWorkerRepository
	clientDbRepo doScreeningWorkerClientDbRepository
	usecase      doScreeningWorkerCSUsecase
}

func NewDoScreeningWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo doScreeningWorkerRepository,
	clientDbRepo doScreeningWorkerClientDbRepository,
	uc doScreeningWorkerCSUsecase,
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
			_, err = w.usecase.HandleCaseCreation(
				ctx,
				tx,
				config,
				monitoredObject.ObjectId,
				continuousScreeningWithMatches,
			)
			return err
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
