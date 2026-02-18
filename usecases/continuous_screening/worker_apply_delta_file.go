// TODO: Implement the delta file update worker, create a stub for now to test the workflow
package continuous_screening

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	MaxBatchSizePerIteration = 100
)

var AllowedRecordOperations = []models.OpenSanctionsDeltaFileRecordOp{
	models.OpenSanctionsDeltaFileRecordOpAdd,
	models.OpenSanctionsDeltaFileRecordOpMod,
}

var AllowedSchemaTypes = []string{
	"Person",
	"Company",
	"Organization",
	"Vessel",
	"Airplane",
}

type applyDeltaFileWorkerRepository interface {
	GetEnrichedContinuousScreeningUpdateJob(
		ctx context.Context,
		exec repositories.Executor,
		updateId uuid.UUID,
	) (models.EnrichedContinuousScreeningUpdateJob, error)
	UpdateContinuousScreeningUpdateJob(
		ctx context.Context,
		exec repositories.Executor,
		updateId uuid.UUID,
		status models.ContinuousScreeningUpdateJobStatus,
	) error

	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID uuid.UUID,
		fetchEnumValues bool,
		useCache bool,
	) (models.DataModel, error)
	GetContinuousScreeningJobOffset(
		ctx context.Context,
		exec repositories.Executor,
		updateJobId uuid.UUID,
	) (*models.ContinuousScreeningJobOffset, error)
	UpsertContinuousScreeningJobOffset(
		ctx context.Context,
		exec repositories.Executor,
		offset models.CreateContinuousScreeningJobOffset,
	) error
	CreateContinuousScreeningJobError(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreeningJobError,
	) error

	SearchScreeningMatchWhitelistByIds(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		counterpartyIds, entityIds []string,
	) ([]models.ScreeningWhitelist, error)

	InsertContinuousScreening(
		ctx context.Context,
		exec repositories.Executor,
		input models.CreateContinuousScreening,
	) (models.ContinuousScreeningWithMatches, error)
}

type applyDeltaFileWorkerTaskQueueRepository interface {
	EnqueueContinuousScreeningMatchEnrichmentTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		continuousScreeningId uuid.UUID,
	) error
}

type applyDeltaFileWorkerScreeningProvider interface {
	Search(
		ctx context.Context,
		query models.OpenSanctionsQuery,
	) (models.ScreeningRawSearchResponseWithMatches, error)
}

type applyDeltaFileWorkerUsecase interface {
	HandleCaseCreation(
		ctx context.Context,
		tx repositories.Transaction,
		config models.ContinuousScreeningConfig,
		objectId string,
		continuousScreeningWithMatches models.ContinuousScreeningWithMatches,
	) (models.Case, error)
	CheckFeatureAccess(ctx context.Context, orgId uuid.UUID) error
}

type ApplyDeltaFileWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningApplyDeltaFileArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	repository         applyDeltaFileWorkerRepository
	taskQueueRepo      applyDeltaFileWorkerTaskQueueRepository
	blobRepository     repositories.BlobRepository
	screeningProvider  applyDeltaFileWorkerScreeningProvider
	usecase            applyDeltaFileWorkerUsecase
	bucketUrl          string
}

func NewApplyDeltaFileWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository applyDeltaFileWorkerRepository,
	taskQueueRepo applyDeltaFileWorkerTaskQueueRepository,
	blobRepository repositories.BlobRepository,
	screeningProvider applyDeltaFileWorkerScreeningProvider,
	bucketUrl string,
	usecase applyDeltaFileWorkerUsecase,
) *ApplyDeltaFileWorker {
	return &ApplyDeltaFileWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repository:         repository,
		taskQueueRepo:      taskQueueRepo,
		blobRepository:     blobRepository,
		screeningProvider:  screeningProvider,
		bucketUrl:          bucketUrl,
		usecase:            usecase,
	}
}

func (w *ApplyDeltaFileWorker) Timeout(job *river.Job[models.ContinuousScreeningApplyDeltaFileArgs]) time.Duration {
	return 10 * time.Minute
}

// Process the delta file, record by record sequentially.
// Could be slow, need to monitor the time process and see if we need to parallelize it
func (w *ApplyDeltaFileWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningApplyDeltaFileArgs]) error {
	logger := utils.LoggerFromContext(ctx)
	exec := w.executorFactory.NewExecutor()

	logger.DebugContext(
		ctx,
		"Starting continuous screening apply delta file update",
		"update_id", job.Args.UpdateId,
		"org_id", job.Args.OrgId,
	)

	if err := w.usecase.CheckFeatureAccess(ctx, job.Args.OrgId); err != nil {
		logger.WarnContext(ctx, "Continuous Screening - feature access not allowed, skipping apply delta file update", "error", err)
		return nil
	}

	updateJob, err := w.repository.GetEnrichedContinuousScreeningUpdateJob(ctx,
		exec, job.Args.UpdateId)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Enriched continuous screening update job", "update_job", updateJob)

	if updateJob.Status == models.ContinuousScreeningUpdateJobStatusCompleted {
		logger.DebugContext(ctx, "Continuous screening update job already completed, skip processing")
		return nil
	}

	initialOffset, initialItemsProcessed, err := w.getLastIterationOffset(
		ctx,
		exec,
		updateJob.Id,
	)
	if err != nil {
		return err
	}

	blob, err := w.blobRepository.GetBlob(
		ctx,
		w.bucketUrl,
		updateJob.DatasetUpdate.DeltaFilePath,
		repositories.WithBeginOffset(initialOffset),
	)
	if err != nil {
		hErr := w.handleProcessError(ctx, exec, job, errors.Wrap(err,
			"failed to get blob"), true)
		if hErr != nil {
			return hErr
		}
		return err
	}
	defer blob.ReadCloser.Close()
	jsonReader := json.NewDecoder(blob.ReadCloser)

	iteration := 0
	for {
		if iteration > 0 && iteration%MaxBatchSizePerIteration == 0 {
			// Save the progress
			err = w.repository.UpsertContinuousScreeningJobOffset(
				ctx,
				exec,
				models.CreateContinuousScreeningJobOffset{
					UpdateJobId:    updateJob.Id,
					ByteOffset:     initialOffset + jsonReader.InputOffset(),
					ItemsProcessed: initialItemsProcessed + iteration,
				},
			)
			if err != nil {
				return err
			}
		}

		var recordHttp httpmodels.HTTPOpenSanctionsDeltaFileRecord
		err := jsonReader.Decode(&recordHttp)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			hErr := w.handleProcessError(ctx, exec, job,
				errors.Wrap(err, "failed to decode record"), false)
			if hErr != nil {
				return hErr
			}
			// Don't need to retry if the record is not valid
			return nil
		}

		record := httpmodels.AdaptOpenSanctionDeltaFileRecordToModel(recordHttp)
		logger = logger.With("name", record.Entity.Caption,
			"record_id", record.Entity.Id,
			"record_op", record.Op.String(),
			"record_schema", record.Entity.Schema)
		ctx = utils.StoreLoggerInContext(ctx, logger)
		iteration++

		if !slices.Contains(AllowedRecordOperations, record.Op) {
			logger.DebugContext(ctx, "Skipping record because op is not allowed")
			continue
		}
		if !slices.Contains(AllowedSchemaTypes, record.Entity.Schema) {
			logger.DebugContext(ctx, "Skipping record because schema is not allowed")
			continue
		}
		// The new record should be in a dataset that is monitored
		if !AtLeastOneDatasetsAreMonitored(record.Entity.Datasets, updateJob.Config.Datasets) {
			logger.DebugContext(ctx, "Skipping record because none of its datasets are monitored", "datasets", record.Entity.Datasets)
			continue
		}

		query, err := w.buildOpenSanctionQuery(ctx, exec, updateJob, record)
		if err != nil {
			return err
		}
		var screeningResponse models.ScreeningRawSearchResponseWithMatches
		logger.DebugContext(ctx, "Performing screening for record")
		err = retry.Do(
			func() error {
				screeningResponse, err = w.screeningProvider.Search(ctx, query)
				return err
			},
			retry.Attempts(3),
			retry.LastErrorOnly(true),
			retry.Delay(100*time.Millisecond),
			retry.DelayType(retry.BackOffDelay),
			retry.Context(ctx),
		)
		if err != nil {
			return err
		}

		screening := screeningResponse.AdaptScreeningFromSearchResponse(query)
		logger.DebugContext(ctx, "Screening result", "matches", len(screening.Matches))

		// No hit, skip the record
		// Note: I'm not entirely sure that we SHOULD keep no written trace of the request that was made here, keeping this question open for now.
		if screening.Status == models.ScreeningStatusNoHit {
			continue
		}

		err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			entityPayload, err := json.Marshal(record.Entity)
			if err != nil {
				return err
			}
			continuousScreeningWithMatches, err := w.repository.InsertContinuousScreening(
				ctx,
				tx,
				models.CreateContinuousScreening{
					Screening:                 screening,
					Config:                    updateJob.Config,
					OpenSanctionEntityId:      &record.Entity.Id,
					OpenSanctionEntityPayload: entityPayload,
					TriggerType:               models.ContinuousScreeningTriggerTypeDatasetUpdated,
				},
			)
			if err != nil {
				return err
			}

			_, err = w.usecase.HandleCaseCreation(
				ctx,
				tx,
				updateJob.Config,
				record.Entity.Caption, // The case title will be the entity caption
				continuousScreeningWithMatches,
			)
			if err != nil {
				return err
			}

			// Enqueue enrichment task for entity payload and matches
			if err := w.taskQueueRepo.EnqueueContinuousScreeningMatchEnrichmentTask(
				ctx,
				tx,
				updateJob.Config.OrgId,
				continuousScreeningWithMatches.Id,
			); err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	err = w.repository.UpdateContinuousScreeningUpdateJob(
		ctx,
		exec,
		updateJob.Id,
		models.ContinuousScreeningUpdateJobStatusCompleted,
	)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Successfully updated continuous screening update job", "update_job", updateJob)
	return nil
}

func AtLeastOneDatasetsAreMonitored(datasets []string, monitoredDatasets []string) bool {
	for _, dataset := range datasets {
		if slices.Contains(monitoredDatasets, dataset) {
			return true
		}
	}
	return false
}

// Return:
// - the offset of the last iteration
// - the number of items processed at the last iteration
func (w *ApplyDeltaFileWorker) getLastIterationOffset(
	ctx context.Context,
	exec repositories.Executor,
	updateJobId uuid.UUID,
) (int64, int, error) {
	logger := utils.LoggerFromContext(ctx)
	// Get offset of the last iteration if it exists
	jobOffset, err := w.repository.GetContinuousScreeningJobOffset(
		ctx,
		exec,
		updateJobId,
	)
	if err != nil {
		return 0, 0, err
	}
	var initialOffset int64
	if jobOffset != nil {
		initialOffset = jobOffset.ByteOffset
		logger.DebugContext(ctx, "Last iteration offset found", "offset", initialOffset)
	} else {
		initialOffset = 0
		logger.DebugContext(ctx, "No last iteration offset found, starting from the beginning")
	}
	var initialItemsProcessed int
	if jobOffset != nil {
		initialItemsProcessed = jobOffset.ItemsProcessed
	} else {
		initialItemsProcessed = 0
	}
	return initialOffset, initialItemsProcessed, nil
}

func (w *ApplyDeltaFileWorker) buildOpenSanctionQuery(
	ctx context.Context,
	exec repositories.Executor,
	updateJob models.EnrichedContinuousScreeningUpdateJob,
	record models.OpenSanctionsDeltaFileRecord,
) (query models.OpenSanctionsQuery, err error) {
	// Fetch whitelist entries for the entity and all its referent (previous) IDs.
	// We then collect their CounterpartyId values and pass them as WhitelistedEntityIds
	// to exclude those counterparties from screening.
	whitelists, err := w.repository.SearchScreeningMatchWhitelistByIds(
		ctx,
		exec,
		updateJob.OrgId,
		nil,
		append(record.Entity.Referents, record.Entity.Id),
	)
	if err != nil {
		return models.OpenSanctionsQuery{}, err
	}
	whitelistedEntityIds := make([]string, len(whitelists))
	for i, whitelist := range whitelists {
		whitelistedEntityIds[i] = whitelist.CounterpartyId
	}

	// Create the openSanction query
	filters := record.Entity.Properties
	return models.OpenSanctionsQuery{
		OrgConfig: models.OrganizationOpenSanctionsConfig{
			MatchThreshold: updateJob.Config.MatchThreshold,
			MatchLimit:     updateJob.Config.MatchLimit,
		},
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type:    record.Entity.Schema,
				Filters: filters,
			},
		},
		WhitelistedEntityIds: whitelistedEntityIds,
		Scope:                orgCustomDatasetName(updateJob.OrgId),
	}, nil
}

func (w *ApplyDeltaFileWorker) handleProcessError(
	ctx context.Context,
	exec repositories.Executor,
	riverJob *river.Job[models.ContinuousScreeningApplyDeltaFileArgs],
	processError error,
	isRetryable bool,
) error {
	if isRetryable && riverJob.Attempt < riverJob.MaxAttempts {
		// Only record the latest internal error to avoid saving all attempts errors
		return nil
	}
	details := map[string]string{
		"error": processError.Error(),
	}
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return err
	}

	err = w.repository.CreateContinuousScreeningJobError(
		ctx,
		exec,
		models.CreateContinuousScreeningJobError{
			UpdateJobId: riverJob.Args.UpdateId,
			Details:     json.RawMessage(detailsBytes),
		},
	)
	if err != nil {
		return err
	}

	err = w.repository.UpdateContinuousScreeningUpdateJob(
		ctx,
		exec,
		riverJob.Args.UpdateId,
		models.ContinuousScreeningUpdateJobStatusFailed,
	)
	if err != nil {
		return err
	}

	return nil
}
