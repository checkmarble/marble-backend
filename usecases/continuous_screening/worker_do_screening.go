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

type doScreeningWorkerRepository interface {
	GetContinuousScreeningConfigByStableId(
		ctx context.Context,
		exec repositories.Executor,
		stableId uuid.UUID,
	) (models.ContinuousScreeningConfig, error)

	InsertContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreening,
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

	// Dataset files
	CreateContinuousScreeningDeltaTrack(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningDeltaTrack,
	) error
}

type doScreeningWorkerClientDbRepository interface {
	GetMonitoredObject(
		ctx context.Context,
		clientExec repositories.Executor,
		monitoringId uuid.UUID,
	) (models.ContinuousScreeningMonitoredObject, error)
}

type doScreeningWorkerIngestedDataReader interface {
	QueryIngestedObjectByInternalId(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		internalObjectId uuid.UUID,
		metadataFields ...string,
	) (models.DataModelObject, error)
}

type doScreeningWorkerCSUsecase interface {
	GetDataModelTableAndMapping(ctx context.Context, exec repositories.Executor,
		config models.ContinuousScreeningConfig, objectType string,
	) (models.Table, models.ContinuousScreeningDataModelMapping, error)
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
	CheckFeatureAccess(ctx context.Context, orgId uuid.UUID) error
}

// Worker to do the screening for a specific monitored object
type DoScreeningWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningDoScreeningArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo               doScreeningWorkerRepository
	clientDbRepo       doScreeningWorkerClientDbRepository
	ingestedDataReader doScreeningWorkerIngestedDataReader
	usecase            doScreeningWorkerCSUsecase
}

func NewDoScreeningWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo doScreeningWorkerRepository,
	clientDbRepo doScreeningWorkerClientDbRepository,
	ingestedDataReader doScreeningWorkerIngestedDataReader,
	uc doScreeningWorkerCSUsecase,
) *DoScreeningWorker {
	return &DoScreeningWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		clientDbRepo:       clientDbRepo,
		ingestedDataReader: ingestedDataReader,
		usecase:            uc,
	}
}

func (w *DoScreeningWorker) Timeout(job *river.Job[models.ContinuousScreeningDoScreeningArgs]) time.Duration {
	return 10 * time.Second
}

// ⚠️ Only Trigger Type = Updated is supported for now, the other trigger types are doing synchonously and don't call this worker
// Work executes the continuous screening process for a specific monitored object.
// The flow consists of the following steps:
//  1. Retrieve the monitored object details from the client's database which contains the object ID and the screening configuration ID.
//  2. Fetch the associated continuous screening configuration.
//  3. Determine the data model table and field mapping for the object type for opensanction query.
//  4. Fetch both the new and previous versions of the ingested object data from the database.
//  5. For update triggers, compare the new data with the previous data for fields mapped to Follow The Money (FTM) properties.
//     If unchanged, skip screening to avoid redundant processing.
//  6. Fetch the latest screening result for the object and check if the existing screening is more recent than the ingested object's valid_from timestamp.
//     If so, skip screening to avoid redundant screening on unchanged data.
//  7. Perform the screening against the configured watchlist/rules.
//  8. If the trigger is an object update, check if the screening results (matches) have changed compared to the latest in review and attached with a case screening result.
//     If unchanged, case creation is skipped to avoid redundant case creation.
//  9. Persist the screening results and, if applicable (and not skipped), handle case creation within a transaction.
func (w *DoScreeningWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningDoScreeningArgs]) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	if err := w.usecase.CheckFeatureAccess(ctx, job.Args.OrgId); err != nil {
		logger.WarnContext(ctx, "Continuous Screening - feature access not allowed, skipping screening", "error", err)
		return nil
	}

	if job.Args.TriggerType != models.ContinuousScreeningTriggerTypeObjectUpdated {
		logger.WarnContext(ctx, "Continuous Screening - only trigger type ObjectUpdated is supported for now, skipping screening", "trigger_type", job.Args.TriggerType)
		return nil
	}

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	newObjectInternalId, err := uuid.Parse(job.Args.NewInternalId)
	if err != nil {
		logger.WarnContext(ctx, "Continuous Screening - could not parse new internal id, skipping screening", "error", err)
		return nil
	}
	previousObjectInternalId, err := uuid.Parse(job.Args.PreviousInternalId)
	if err != nil {
		logger.WarnContext(ctx, "Continuous Screening - could not parse previous internal id, skipping screening", "error", err)
		return nil
	}

	// Fetch the monitored object from client DB
	monitoredObject, err := w.clientDbRepo.GetMonitoredObject(
		ctx,
		clientDbExec,
		job.Args.MonitoringId,
	)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			logger.WarnContext(ctx, "Continuous Screening - monitored object not found, skipping screening",
				"monitoring_id", job.Args.MonitoringId)
			// No need to retry the job
			return nil
		}
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
	configuredFields := table.GetFieldsWithFTMProperty()

	newObjectData, err := w.ingestedDataReader.QueryIngestedObjectByInternalId(ctx, clientDbExec, table,
		newObjectInternalId, "id", "valid_from")
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			logger.WarnContext(ctx, "Continuous Screening - new object data not found, skipping screening", "new_internal_id", newObjectInternalId)
		}
		return err

	}

	// Get list of fields configured for continuous screening
	previousObjectData, err := w.ingestedDataReader.QueryIngestedObjectByInternalId(
		ctx, clientDbExec, table, previousObjectInternalId)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			logger.WarnContext(ctx, "Continuous Screening - previous object data not found, skipping screening",
				"previous_internal_id", previousObjectInternalId)
			return nil
		}
		return err
	}

	// Check if the previous object data is the same as the new object data, if yes, skip screening
	if areObjectsEqual(previousObjectData, newObjectData, configuredFields) {
		logger.InfoContext(ctx, "Continuous Screening - previous object data is the same as the new object data, skipping screening",
			"previous_internal_id", previousObjectInternalId,
			"new_internal_id", newObjectInternalId)
		return nil
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
		ingestedObjectValidFrom, ok := newObjectData.Metadata["valid_from"].(time.Time)
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
		newObjectData,
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
			models.CreateContinuousScreening{
				Screening:        screeningWithMatches,
				Config:           config,
				ObjectType:       &job.Args.ObjectType,
				ObjectId:         &monitoredObject.ObjectId,
				ObjectInternalId: &newObjectInternalId,
				TriggerType:      job.Args.TriggerType,
			},
		)
		if err != nil {
			return err
		}

		err = w.repo.CreateContinuousScreeningDeltaTrack(ctx, tx, models.CreateContinuousScreeningDeltaTrack{
			OrgId:            config.OrgId,
			ObjectType:       job.Args.ObjectType,
			ObjectId:         monitoredObject.ObjectId,
			ObjectInternalId: &newObjectInternalId,
			EntityId:         marbleEntityIdBuilder(job.Args.ObjectType, monitoredObject.ObjectId),
			Operation:        models.DeltaTrackOperationUpdate,
		})
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

func areObjectsEqual(previousObjectData models.DataModelObject,
	newObjectData models.DataModelObject, configuredFields []models.Field,
) bool {
	for _, field := range configuredFields {
		valPrev := previousObjectData.Data[field.Name]
		valNew := newObjectData.Data[field.Name]

		// == compares Location and monotonic clock, .Equal() compares the instant.
		tPrev, okPrev := valPrev.(time.Time)
		tNew, okNew := valNew.(time.Time)
		if okPrev && okNew {
			if !tPrev.Equal(tNew) {
				return false
			}
			continue
		}

		if valPrev != valNew {
			return false
		}
	}
	return true
}
