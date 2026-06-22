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
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/repositories/screening"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"github.com/tidwall/gjson"
)

const (
	MaxBatchSizePerIteration = 50
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
	"LegalEntity",
}

type applyDeltaFileWorkerRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)

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
		providerName models.ScreeningProvider,
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
	AdaptLegacyDatasets(ctx context.Context, config models.ContinuousScreeningConfig) (models.ContinuousScreeningConfig, error)
}

type ApplyDeltaFileWorker struct {
	river.WorkerDefaults[models.ContinuousScreeningApplyDeltaFileArgs]

	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	redisClient        *repositories.RedisClient
	repository         applyDeltaFileWorkerRepository
	taskQueueRepo      applyDeltaFileWorkerTaskQueueRepository
	blobRepository     repositories.BlobRepository
	screeningProvider  applyDeltaFileWorkerScreeningProvider
	usecase            applyDeltaFileWorkerUsecase
	offloadedReader    repositories.OffloadedReadWriter
	bucketUrl          string
}

func NewApplyDeltaFileWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	redisClient *repositories.RedisClient,
	repository applyDeltaFileWorkerRepository,
	taskQueueRepo applyDeltaFileWorkerTaskQueueRepository,
	blobRepository repositories.BlobRepository,
	screeningProvider applyDeltaFileWorkerScreeningProvider,
	bucketUrl string,
	usecase applyDeltaFileWorkerUsecase,
	offloadedReader repositories.OffloadedReadWriter,
) *ApplyDeltaFileWorker {
	return &ApplyDeltaFileWorker{
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		redisClient:        redisClient,
		repository:         repository,
		taskQueueRepo:      taskQueueRepo,
		blobRepository:     blobRepository,
		screeningProvider:  screeningProvider,
		bucketUrl:          bucketUrl,
		usecase:            usecase,
		offloadedReader:    offloadedReader,
	}
}

func (w *ApplyDeltaFileWorker) Timeout(job *river.Job[models.ContinuousScreeningApplyDeltaFileArgs]) time.Duration {
	return 10 * time.Minute
}

// Process the delta file, record by record sequentially.
// Could be slow, need to monitor the time process and see if we need to parallelize it
func (w *ApplyDeltaFileWorker) Work(ctx context.Context, job *river.Job[models.ContinuousScreeningApplyDeltaFileArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	if utils.GetEnv("DISABLE_CONTINUOUS_SCREENING_APPLY_DELTA_FILE", false) {
		logger.InfoContext(ctx, "Continuous screening apply delta file is disabled, skipping job",
			"update_id", job.Args.UpdateId,
			"org_id", job.Args.OrgId)
		return nil
	}

	exec := w.executorFactory.NewExecutor()

	// Log error if job has been retried many times
	if gjson.GetBytes(job.Metadata, "snoozes").Int() > 100 {
		logger.ErrorContext(ctx,
			"Continuous screening apply delta file job has exceeded 100 attempts",
			"attempt", job.Attempt,
			"update_id", job.Args.UpdateId,
			"org_id", job.Args.OrgId)
	}

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

	// Adapt legacy datasets to the new format for the the filtering which will be used in matchesFilters method
	updateJob.Config, err = w.usecase.AdaptLegacyDatasets(ctx, updateJob.Config)
	if err != nil {
		return err
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

	redisExec := w.redisClient.NewExecutor(job.Args.OrgId, "deltas")

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
		// create a logger for the iteration, because structured log fields accumulate (they don't replace the existing)
		iterLogger := logger.With("name", record.Entity.Caption,
			"record_id", record.Entity.Id,
			"record_op", record.Op.String(),
			"record_schema", record.Entity.Schema)
		iterCtx := utils.StoreLoggerInContext(ctx, iterLogger)
		iteration++

		if !slices.Contains(AllowedRecordOperations, record.Op) {
			iterLogger.DebugContext(iterCtx, "Skipping record because op is not allowed")
			continue
		}
		if !slices.Contains(AllowedSchemaTypes, record.Entity.Schema) {
			iterLogger.DebugContext(iterCtx, "Skipping record because schema is not allowed")
			continue
		}

		if !matchesFilters(updateJob, record) {
			iterLogger.DebugContext(iterCtx, "Skipping record because it does not meet filter")
			continue
		}

		if record.Entity.LastChange != nil {
			previousChangeAt, err := repositories.RedisGetScalar[time.Time](ctx, redisExec, redisExec.Key(record.Entity.Id))
			if err == nil {
				if previousChangeAt.Equal(*record.Entity.LastChange) || record.Entity.LastChange.Before(previousChangeAt) {
					iterLogger.DebugContext(iterCtx, "Skipping record because we already processed this version")
					continue
				}
			}
		}

		savedLastChange := func() {
			if record.Entity.LastChange != nil {
				err := redisExec.Exec(func(c *redis.Client) error {
					return c.Set(ctx, redisExec.Key(record.Entity.Id), record.Entity.LastChange.Format(time.RFC3339Nano), 0).Err()
				})
				if err != nil {
					iterLogger.WarnContext(iterCtx, "could not save last_change for entity to redis")
				}
			}
		}

		query, err := w.buildOpenSanctionQuery(iterCtx, exec, updateJob, record)
		if err != nil {
			return err
		}

		var screeningResponse models.ScreeningRawSearchResponseWithMatches
		iterLogger.DebugContext(iterCtx, "Performing screening for record")
		err = retry.Do(
			func() error {
				// Searches in this direction should use the Open Sanctions schema (no complex topics filters).
				screeningResponse, err = w.screeningProvider.Search(iterCtx, models.ScreeningProviderOpenSanctions, query)
				return err
			},
			retry.Attempts(3),
			retry.LastErrorOnly(true),
			retry.Delay(100*time.Millisecond),
			retry.DelayType(retry.BackOffDelay),
			retry.Context(iterCtx),
		)
		if err != nil {
			// Handle transient screening API errors (408 timeout, 502 bad gateway) gracefully
			if isTransientScreeningError(err) {
				iterLogger.WarnContext(iterCtx, "Screening API transient error, rescheduling job", "error", err.Error())
				return river.JobSnooze(5 * time.Minute)
			}
			return err
		}

		screening := screeningResponse.AdaptScreeningFromSearchResponse(query)
		iterLogger.DebugContext(iterCtx, "Screening result", "matches", len(screening.Matches))

		entityPayload, err := json.Marshal(record.Entity)
		if err != nil {
			return err
		}

		createInput := models.CreateContinuousScreening{
			Id:                        pure_utils.NewId(),
			Screening:                 screening,
			Config:                    updateJob.Config,
			OpenSanctionEntityId:      &record.Entity.Id,
			OpenSanctionEntityPayload: entityPayload,
			TriggerType:               models.ContinuousScreeningTriggerTypeDatasetUpdated,
		}

		// Offload only the entity payload to blob storage (no-op when offloading is disabled).
		// Match payloads are customer data and are kept in the DB column in this direction.
		createInput.OpenSanctionEntityPayload, err = w.offloadedReader.OffloadContinuousScreeningEntity(
			iterCtx, updateJob.Config.OrgId, createInput.Id, createInput.OpenSanctionEntityPayload)
		if err != nil {
			return errors.Wrap(err, "failed to offload continuous screening entity payload")
		}

		err = w.transactionFactory.Transaction(iterCtx, func(tx repositories.Transaction) error {
			continuousScreeningWithMatches, err := w.repository.InsertContinuousScreening(iterCtx, tx, createInput)
			if err != nil {
				return err
			}
			// if there were no hits, that's it
			if screening.Status == models.ScreeningStatusNoHit {
				return nil
			}

			_, err = w.usecase.HandleCaseCreation(
				iterCtx,
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
				iterCtx,
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

		savedLastChange()
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

	// Upstream -> Organization record searches should not contain topics filters
	delete(filters, "topics")
	// programId is reserved on org-dataset entities to carry the source table name;
	// strip it from the incoming record's scoring properties to avoid noise.
	delete(filters, "programId")

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
		Partition:            true,
		ObjectTypes:          updateJob.Config.ObjectTypes,
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

// isTransientScreeningError checks if an error is a transient screening API error
// (408, 429, 502, 503, 504) that should trigger a job snooze rather than a permanent failure
func isTransientScreeningError(err error) bool {
	var httpErr *screening.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.IsTransient()
	}
	return false
}

// Topics come in groups, matching is a AND of groups with an OR inside groups.
func topicsMatchForSection(sectionTopics map[string][]string, record models.OpenSanctionsDeltaFileRecord) bool {
	for _, topicGroup := range sectionTopics {
		topicFound := false

		for _, topic := range record.Entity.Properties["topics"] {
			if slices.Contains(topicGroup, topic) {
				topicFound = true
				break
			}
		}

		if !topicFound {
			return false
		}
	}
	return true
}

func matchesFilters(updateJob models.EnrichedContinuousScreeningUpdateJob, record models.OpenSanctionsDeltaFileRecord) bool {
	filters := updateJob.Config.Filters.Resolve()

	switch updateJob.Config.Provider {
	case models.ScreeningProviderOpenSanctions:
		ds := append(filters.Sanctions.Datasets, filters.Peps.Datasets...)
		ds = append(ds, filters.AdverseMedia.Datasets...)
		ds = append(ds, filters.Other.Datasets...)
		ds = append(ds, filters.Custom.Datasets...)

		return AtLeastOneDatasetsAreMonitored(record.Entity.Datasets, ds)

	case models.ScreeningProviderLexisNexis:
		// Loop over each configuration section
		globalFilter := filters.Global
		if globalFilter.Enabled {
			if !topicsMatchForSection(globalFilter.Topics, record) {
				return false
			}
		}

		recordMatches := false
		for rootTopic, section := range filters.WithRootTopics() {
			// Short-circuit if the section is not enabled. Global filters ("is alive"...) are applied before the loop, separately because all records must match them regardless of their root topic.
			if !section.Enabled || rootTopic == "global" {
				continue
			}

			sectionMatches := true

			// If we searches for specific datasets (programId for Lexis Nexis), break if not found
			if len(section.Datasets) > 0 {
				foundDataset := false
				for _, recordDataset := range record.Entity.Properties["programId"] {
					if slices.Contains(section.Datasets, recordDataset) {
						foundDataset = true
						break
					}
				}

				if !foundDataset {
					sectionMatches = false
				}
			}

			// Check whether the record holds the root topic for the section
			if rootTopic != "other" && !slices.Contains(record.Entity.Properties["topics"], rootTopic) {
				sectionMatches = false
			}

			if !topicsMatchForSection(section.Topics, record) {
				sectionMatches = false
			}

			if sectionMatches {
				recordMatches = true
				break
			}
		}

		return recordMatches
	}

	return false
}
