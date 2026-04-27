package continuous_screening

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type registerObjectWorkerRepository interface {
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
	CreateContinuousScreeningDeltaTrack(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningDeltaTrack,
	) error
}

type registerObjectWorkerTaskQueueRepository interface {
	EnqueueContinuousScreeningMatchEnrichmentTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		continuousScreeningId uuid.UUID,
	) error
}

type registerObjectWorkerClientDbRepository interface {
	InsertContinuousScreeningObject(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectId string,
		configStableId uuid.UUID,
	) (uuid.UUID, error)
	InsertContinuousScreeningAudit(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningAudit,
	) error
	ListMonitoredObjectsByObjectIds(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectIds []string,
	) ([]models.ContinuousScreeningMonitoredObject, error)
}

type registerObjectWorkerIngestedDataReader interface {
	QueryIngestedObjectByInternalId(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		internalObjectId uuid.UUID,
		metadataFields ...string,
	) (models.DataModelObject, error)
}

type registerObjectWorkerCSUsecase interface {
	GetDataModelTableAndMapping(
		ctx context.Context,
		exec repositories.Executor,
		config models.ContinuousScreeningConfig,
		objectType string,
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
		caseName string,
		continuousScreeningWithMatches models.ContinuousScreeningWithMatches,
	) (models.Case, error)
	CheckFeatureAccess(ctx context.Context, orgId uuid.UUID) error
}

// RegisterObjectWorker handles newly monitored objects coming through the ingestion path.
// It inserts the object into the monitoring table, creates an audit entry, records an Add
// delta track, and optionally performs the initial screening.
type RegisterObjectWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningRegisterObjectArgs]
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory

	repo               registerObjectWorkerRepository
	taskQueueRepo      registerObjectWorkerTaskQueueRepository
	clientDbRepo       registerObjectWorkerClientDbRepository
	ingestedDataReader registerObjectWorkerIngestedDataReader
	usecase            registerObjectWorkerCSUsecase
}

func NewRegisterObjectWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repo registerObjectWorkerRepository,
	taskQueueRepo registerObjectWorkerTaskQueueRepository,
	clientDbRepo registerObjectWorkerClientDbRepository,
	ingestedDataReader registerObjectWorkerIngestedDataReader,
	uc registerObjectWorkerCSUsecase,
) *RegisterObjectWorker {
	return &RegisterObjectWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repo:               repo,
		taskQueueRepo:      taskQueueRepo,
		clientDbRepo:       clientDbRepo,
		ingestedDataReader: ingestedDataReader,
		usecase:            uc,
	}
}

func (w *RegisterObjectWorker) Timeout(_ *river.Job[models.ContinuousScreeningRegisterObjectArgs]) time.Duration {
	return time.Minute
}

// Work registers a newly ingested object under continuous screening monitoring.
// The flow has two independent steps, each of which is safe to retry:
//  1. Registration (client DB transaction): if the (objectId, configId) pair is not already
//     in the monitoring table, insert it and create an audit entry.
//  2. Delta track (marble DB): record an Add operation so the object is included in the next
//     dataset rebuild. Always attempted — ON CONFLICT DO NOTHING makes it idempotent, so
//     retries and concurrent jobs for the same object are handled by the unique constraint on
//     (org_id, object_type, object_id, object_internal_id) for add/update operations.
//  3. Screening (marble DB transaction, only if ShouldScreen): query OpenSanctions, persist
//     the result, enqueue match enrichment, and create a case if the result is in-review.
//     This step is always attempted — even on retry — so a failed screening transaction does
//     not leave the object permanently unscreened.
func (w *RegisterObjectWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningRegisterObjectArgs]) error {
	exec := w.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	var userId *uuid.UUID
	if job.Args.UserId != nil {
		parsed, err := uuid.Parse(*job.Args.UserId)
		if err != nil {
			return river.JobCancel(errors.New("could not parse user_id"))
		}
		userId = &parsed
	}
	var apiKeyId *uuid.UUID
	if job.Args.ApiKeyId != nil {
		parsed, err := uuid.Parse(*job.Args.ApiKeyId)
		if err != nil {
			return river.JobCancel(errors.New("could not parse api_key_id"))
		}
		apiKeyId = &parsed
	}

	if err := w.usecase.CheckFeatureAccess(ctx, job.Args.OrgId); err != nil {
		logger.WarnContext(ctx, "Continuous Screening - feature access not allowed, skipping registration", "error", err)
		return nil
	}

	newObjectInternalId, err := uuid.Parse(job.Args.NewInternalId)
	if err != nil {
		return river.JobCancel(errors.New("could not parse new internal id"))
	}

	config, err := w.repo.GetContinuousScreeningConfigByStableId(ctx, exec, job.Args.ConfigStableId)
	if err != nil {
		return err
	}

	table, mapping, err := w.usecase.GetDataModelTableAndMapping(ctx, exec, config, job.Args.ObjectType)
	if err != nil {
		return err
	}

	clientDbExec, err := w.executorFactory.NewClientDbExecutor(ctx, job.Args.OrgId)
	if err != nil {
		return err
	}

	// Check if the ingested object exists
	newObjectData, err := w.ingestedDataReader.QueryIngestedObjectByInternalId(
		ctx, clientDbExec, table, newObjectInternalId, "id", "valid_from",
	)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			logger.WarnContext(ctx, "Continuous Screening - ingested object not found", "new_internal_id", newObjectInternalId)
		}
		return err
	}

	// Check if already registered, insert if not, create audit.
	err = w.transactionFactory.TransactionInOrgSchema(ctx, job.Args.OrgId, func(tx repositories.Transaction) error {
		monitoredObjects, err := w.clientDbRepo.ListMonitoredObjectsByObjectIds(
			ctx, tx, table.Name, []string{job.Args.ObjectId},
		)
		if err != nil {
			return err
		}

		for _, mo := range monitoredObjects {
			if mo.ConfigStableId == job.Args.ConfigStableId {
				return nil
			}
		}

		if _, err := w.clientDbRepo.InsertContinuousScreeningObject(
			ctx, tx, table.Name, job.Args.ObjectId, job.Args.ConfigStableId,
		); err != nil {
			return err
		}

		return w.clientDbRepo.InsertContinuousScreeningAudit(ctx, tx, models.CreateContinuousScreeningAudit{
			ObjectType:     table.Name,
			ObjectId:       job.Args.ObjectId,
			ConfigStableId: job.Args.ConfigStableId,
			Action:         models.ContinuousScreeningAuditActionAdd,
			UserId:         userId,
			ApiKeyId:       apiKeyId,
		})
	})
	if err != nil {
		return err
	}

	// Always attempt the delta track insert — ON CONFLICT DO NOTHING makes it idempotent.
	// This handles retries (alreadyRegistered) and concurrent jobs for the same object.
	if err := w.repo.CreateContinuousScreeningDeltaTrack(ctx, exec, models.CreateContinuousScreeningDeltaTrack{
		OrgId:            config.OrgId,
		ObjectType:       job.Args.ObjectType,
		ObjectId:         job.Args.ObjectId,
		ObjectInternalId: &newObjectInternalId,
		EntityId:         pure_utils.MarbleEntityIdBuilder(job.Args.ObjectType, job.Args.ObjectId),
		Operation:        models.DeltaTrackOperationAdd,
	}); err != nil {
		return err
	}

	if !job.Args.ShouldScreen {
		return nil
	}

	screeningWithMatches, err := w.usecase.DoScreening(
		ctx, exec, newObjectData, mapping, config, job.Args.ObjectType, job.Args.ObjectId,
	)
	if err != nil {
		return err
	}

	return w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		continuousScreeningWithMatches, err := w.repo.InsertContinuousScreening(ctx, tx, models.CreateContinuousScreening{
			Screening:        screeningWithMatches,
			Config:           config,
			ObjectType:       &job.Args.ObjectType,
			ObjectId:         &job.Args.ObjectId,
			ObjectInternalId: &newObjectInternalId,
			TriggerType:      models.ContinuousScreeningTriggerTypeObjectAdded,
		})
		if err != nil {
			return err
		}

		if err := w.taskQueueRepo.EnqueueContinuousScreeningMatchEnrichmentTask(
			ctx, tx, config.OrgId, continuousScreeningWithMatches.Id,
		); err != nil {
			return err
		}

		if screeningWithMatches.Status == models.ScreeningStatusInReview {
			caseName, err := buildCaseName(newObjectData, mapping)
			if err != nil {
				logger.WarnContext(ctx, "Continuous Screening - error building case name, falling back to object_id", "error", err)
				caseName = job.Args.ObjectId
			}
			if _, err = w.usecase.HandleCaseCreation(ctx, tx, config, caseName, continuousScreeningWithMatches); err != nil {
				return err
			}
		}

		return nil
	})
}
