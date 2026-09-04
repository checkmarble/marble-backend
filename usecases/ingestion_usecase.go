package usecases

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river"
	"github.com/tidwall/gjson"
	"github.com/twpayne/go-geom"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	DefaultApiBatchIngestionSize = 100

	CSV_INGESTION_ITERATION_TIMEOUT = 10 * time.Second

	// CSV_INGESTION_TIMEOUT_MARGIN is the headroom reserved before river's own Timeout, so there is
	// time to save the checkpoint and snooze before river cancels the job. The deadline is only
	// checked once a batch has committed, so the margin must cover one more full iteration: reading
	// csvIngestionBatchSize rows off the blob reader (bounded by nothing but river's cancellation),
	// one ingestion batch (bounded by CSV_INGESTION_ITERATION_TIMEOUT, which retryIngestion's two
	// attempts share as a single deadline), and one checkpoint write.
	CSV_INGESTION_TIMEOUT_MARGIN = 2 * time.Minute
	CSV_INGESTION_SNOOZE_DELAY   = 5 * time.Second
)

var csvIngestionBatchSize = utils.GetEnv("CSV_INGESTION_BATCH_SIZE", 1000)

// csvIngestionMaxSnoozes bounds how many times a single upload may be resumed before we give up on
// it. It is coupled to CSV_INGESTION_TIMEOUT (default 1h), so the default is roughly a 24h ceiling:
// change one and revisit the other.
var csvIngestionMaxSnoozes = utils.GetEnv("CSV_INGESTION_MAX_SNOOZES", 24)
var csvIngestionMaxRetries = utils.GetEnv("CSV_INGESTION_MAX_RETRIES", 12)
var csvIngestionTotalTimeout = utils.GetEnvDuration("CSV_INGESTION_TOTAL_TIMEOUT", models.CsvIngestionTotalTimeoutDefault)

type continuousScreeningRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)
	ListContinuousScreeningConfigByObjectType(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		provider models.ScreeningProvider,
		objectType string,
	) ([]models.ContinuousScreeningConfig, error)
	ListContinuousScreeningConfigByStableIds(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		provider models.ScreeningProvider,
		stableIds []uuid.UUID,
	) ([]models.ContinuousScreeningConfig, error)
}

type continuousScreeningClientDbRepository interface {
	ListMonitoredObjectsByObjectIds(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectIds []string,
	) ([]models.ContinuousScreeningMonitoredObject, error)
	IsContinuousScreeningSetup(ctx context.Context, exec repositories.Executor) (bool, error)
}

type taskEnqueuer interface {
	EnqueueContinuousScreeningDoScreeningTaskMany(
		ctx context.Context,
		tx repositories.Transaction,
		orgId uuid.UUID,
		objectType string,
		enqueueObjectUpdateTasks []models.ContinuousScreeningEnqueueObjectUpdateTask,
		triggerType models.ContinuousScreeningTriggerType,
	) error
	EnqueueContinuousScreeningRegisterObjectTaskMany(
		ctx context.Context,
		tx repositories.Transaction,
		orgId uuid.UUID,
		objectType string,
		tasks []models.ContinuousScreeningRegisterObjectTask,
		shouldScreen bool,
	) error
	EnqueueCsvIngestionTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		uploadLogId uuid.UUID,
		ingestionOptions models.IngestionOptions,
	) error
	EnqueueCsvIngestionDeadlineTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		uploadLogId uuid.UUID,
		deadline time.Time,
	) error
	EnqueueTriggerScoreComputation(
		ctx context.Context,
		tx repositories.Transaction,
		record models.ScoringRecordRef,
	) error
	EnqueueManyTriggerScoreComputation(
		ctx context.Context,
		tx repositories.Transaction,
		entities []models.ScoringRecordRef,
	) error
	EnqueueAsyncUploadTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		uploadLogId uuid.UUID,
		objectType string,
		key string,
		ingestionOptions models.IngestionOptions,
	) error
}

type scoreComputationUsecase interface {
	EnqueueComputationForIngestion(ctx context.Context, orgId uuid.UUID, recordType string, records models.IngestionResults) error
}

type ingestionWebhookEventsUsecase interface {
	CreateWebhookEvent(ctx context.Context, tx repositories.Transaction, input models.WebhookEventCreate) error
}

type IngestionUseCase struct {
	transactionFactory                  executor_factory.TransactionFactory
	executorFactory                     executor_factory.ExecutorFactory
	enforceSecurity                     security.EnforceSecurityIngestion
	scoringScoreUsecase                 scoreComputationUsecase
	ingestionRepository                 repositories.IngestionRepository
	blobRepository                      repositories.BlobRepository
	dataModelRepository                 repositories.DataModelRepository
	uploadLogRepository                 repositories.UploadLogRepository
	payloadEnricher                     payload_parser.PayloadEnrichementUsecase
	continuousScreeningRepository       continuousScreeningRepository
	continuousScreeningClientRepository continuousScreeningClientDbRepository
	ingestionBucketUrl                  string
	batchIngestionMaxSize               int
	taskEnqueuer                        taskEnqueuer
	webhookEventsUsecase                ingestionWebhookEventsUsecase
	isManagedMarble                     bool
}

func (usecase *IngestionUseCase) IngestObject(
	ctx context.Context,
	organizationId uuid.UUID,
	objectType string,
	objectBody json.RawMessage,
	ingestionOptions models.IngestionOptions,
	parserOpts ...payload_parser.ParserOpt,
) (int, error) {
	logger := utils.LoggerFromContext(ctx)
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"IngestionUseCase.IngestObject",
		trace.WithAttributes(attribute.String("object_type", objectType)),
		trace.WithAttributes(attribute.String("organization_id", organizationId.String())),
	)
	defer span.End()

	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return 0, err
	}

	exec := usecase.executorFactory.NewExecutor()

	org, err := usecase.continuousScreeningRepository.GetOrganizationById(ctx, exec, organizationId)
	if err != nil {
		return 0, errors.Wrap(err, "error getting organization")
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, true)
	if err != nil {
		return 0, errors.Wrap(err, "error getting data model in IngestObject")
	}

	tables := dataModel.Tables
	table, ok := tables[objectType]
	if !ok {
		return 0, errors.WithDetailf(
			models.NotFoundError,
			"table %s not found in data model in IngestObject", objectType,
		)
	}

	var continuousScreeningConfigs []models.ContinuousScreeningConfig

	if ingestionOptions.ShouldMonitor {
		continuousScreeningConfigs, err = usecase.continuousScreeningRepository.ListContinuousScreeningConfigByStableIds(
			ctx, exec, organizationId, org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring), ingestionOptions.ContinuousScreeningIds,
		)
		if err != nil {
			return 0, err
		}

		if err := validateContinuousScreeningConfigs(continuousScreeningConfigs, ingestionOptions.ContinuousScreeningIds, table.Name); err != nil {
			return 0, err
		}
	}

	parser := payload_parser.NewParser(append(parserOpts, payload_parser.WithColumnEscape(),
		payload_parser.WithEnricher(usecase.payloadEnricher))...)
	payload, err := parser.ParsePayload(ctx, table, objectBody)
	if err != nil {
		return 0, errors.WithDetail(err, "error parsing payload in decision usecase validate payload")
	}

	var ingestionResults models.IngestionResults
	err = retryIngestion(ctx, func() error {
		ingestionResults, err = usecase.insertEnumValuesAndIngest(ctx,
			organizationId, []models.ClientObject{payload}, table, ingestionOptions)
		return err
	})
	if err != nil {
		var validationErrors models.IngestionValidationErrors
		if errors.As(err, &validationErrors) {
			// if err is not nil, the call to the repository may return a models.IngestionValidationErrorsMultiple
			// instance error, in which case it should have just one entry (with the input object_id as key)
			// return 0, models.IngestionValidationErrorsSingle(
			// 	validationErrors[payload.Data["object_id"].(string)])
			return 0, validationErrors
		}
		return 0, err
	}
	nbInsertedObjects := len(ingestionResults)

	logger.DebugContext(
		ctx, fmt.Sprintf("Successfully ingested objects: %d objects", nbInsertedObjects),
		slog.String("organization_id", organizationId.String()),
		slog.String("object_type", objectType),
		slog.Int("nb_objects", nbInsertedObjects),
	)

	return nbInsertedObjects, nil
}

func (usecase *IngestionUseCase) IngestObjects(
	ctx context.Context,
	organizationId uuid.UUID,
	objectType string,
	objectBody json.RawMessage,
	ingestionOptions models.IngestionOptions,
	parserOpts ...payload_parser.ParserOpt,
) (int, error) {
	logger := utils.LoggerFromContext(ctx)
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"IngestionUseCase.IngestObjects",
		trace.WithAttributes(attribute.String("object_type", objectType)),
		trace.WithAttributes(attribute.String("organization_id", organizationId.String())),
	)
	defer span.End()

	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return 0, err
	}

	var rawMessages []json.RawMessage
	if err := json.Unmarshal(objectBody, &rawMessages); err != nil {
		return 0, errors.Wrap(models.BadParameterError,
			"error unmarshalling objectBody in IngestObjects")
	}
	if len(rawMessages) > usecase.batchIngestionMaxSize {
		return 0, errors.WithDetail(models.BadParameterError, "too many objects in the batch")
	}

	exec := usecase.executorFactory.NewExecutor()

	org, err := usecase.continuousScreeningRepository.GetOrganizationById(ctx, exec, organizationId)
	if err != nil {
		return 0, errors.Wrap(err, "error getting organization")
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, true)
	if err != nil {
		return 0, errors.Wrap(err, "error getting data model in IngestObjects")
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return 0, errors.WithDetailf(
			models.NotFoundError,
			"table %s not found in data model in IngestObjects", objectType,
		)
	}

	var continuousScreeningConfigs []models.ContinuousScreeningConfig

	if ingestionOptions.ShouldMonitor {
		continuousScreeningConfigs, err = usecase.continuousScreeningRepository.ListContinuousScreeningConfigByStableIds(
			ctx, exec, organizationId, org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring), ingestionOptions.ContinuousScreeningIds,
		)
		if err != nil {
			return 0, err
		}

		if err := validateContinuousScreeningConfigs(continuousScreeningConfigs, ingestionOptions.ContinuousScreeningIds, table.Name); err != nil {
			return 0, err
		}
	}

	clientObjects := make([]models.ClientObject, 0, len(rawMessages))
	objectIds := make(map[string]struct{}, len(rawMessages))
	parser := payload_parser.NewParser(append(parserOpts, payload_parser.WithColumnEscape(),
		payload_parser.WithEnricher(usecase.payloadEnricher))...)
	validationErrorsGroup := make(models.IngestionValidationErrors)
	for _, rawMsg := range rawMessages {
		payload, err := parser.ParsePayload(ctx, table, rawMsg)
		var validationErrors models.IngestionValidationErrors
		if errors.As(err, &validationErrors) {
			objectId, errMap := validationErrors.GetSomeItem()
			validationErrorsGroup[objectId] = errMap
			continue
		} else if err != nil {
			return 0, errors.WithDetailf(
				models.BadParameterError,
				"Error while validating payload in IngestObjects: %v", err,
			)
		}
		objectId := payload.Data["object_id"].(string)
		if _, ok := objectIds[objectId]; ok {
			return 0, errors.WithDetailf(models.BadParameterError,
				"duplicate object_id %s in the batch", objectId)
		}
		objectIds[objectId] = struct{}{}
		clientObjects = append(clientObjects, payload)
	}
	if len(validationErrorsGroup) > 0 {
		return 0, validationErrorsGroup
	}

	var ingestionResults models.IngestionResults
	err = retryIngestion(ctx, func() error {
		ingestionResults, err = usecase.insertEnumValuesAndIngest(ctx, organizationId,
			clientObjects, table, ingestionOptions)
		return err
	})
	if err != nil {
		return 0, err
	}
	nbInsertedObjects := len(ingestionResults)

	logger.DebugContext(
		ctx, fmt.Sprintf("Successfully ingested objects: %d objects", nbInsertedObjects),
		slog.String("organization_id", organizationId.String()),
		slog.String("object_type", objectType),
		slog.Int("nb_objects", nbInsertedObjects),
	)

	return nbInsertedObjects, nil
}

func (usecase *IngestionUseCase) ListUploadLogs(ctx context.Context,
	organizationId uuid.UUID, objectType string,
) ([]models.UploadLog, error) {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return []models.UploadLog{}, err
	}

	return usecase.uploadLogRepository.AllUploadLogsByTable(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, objectType)
}

func (usecase *IngestionUseCase) ListFilteredUploadLogs(
	ctx context.Context,
	organizationId uuid.UUID, objectType string,
	filters models.UploadLogFilters,
	pagination models.PaginationAndSorting,
) (models.Paginated[models.UploadLog], error) {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return models.Paginated[models.UploadLog]{}, err
	}

	page := models.PaginationAndSorting{
		OffsetId: pagination.OffsetId,
		Sorting:  pagination.Sorting,
		Order:    pagination.Order,
		Limit:    pagination.Limit + 1,
	}

	logs, err := usecase.uploadLogRepository.ListUploadLogs(
		ctx,
		usecase.executorFactory.NewExecutor(), organizationId, objectType,
		filters, page,
	)
	if err != nil {
		return models.Paginated[models.UploadLog]{}, err
	}

	return models.Paginated[models.UploadLog]{
		Items:       logs[:min(len(logs), pagination.Limit)],
		HasNextPage: len(logs) > pagination.Limit,
	}, nil
}

func (usecase *IngestionUseCase) ValidateAndUploadIngestionCsv(ctx context.Context,
	organizationId uuid.UUID, userId, objectType string, fileReader *csv.Reader,
	ingestionOptions models.IngestionOptions,
) (models.UploadLog, error) {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return models.UploadLog{}, err
	}
	dataModel, err := usecase.dataModelRepository.GetDataModel(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		false,
		true,
	)
	if err != nil {
		return models.UploadLog{}, err
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return models.UploadLog{}, fmt.Errorf("table %s not found on data model", objectType)
	}

	headers, err := fileReader.Read()
	if err != nil {
		var csvErr *csv.ParseError

		if errors.As(err, &csvErr) {
			lastColumn := "first header"
			if len(headers) > 0 {
				lastColumn = fmt.Sprintf("header after `%s`", headers[len(headers)-1])
			}

			return models.UploadLog{}, errors.Wrap(models.BadParameterError,
				fmt.Sprintf("error reading CSV %s (column %d): %v",
					lastColumn, csvErr.Column, csvErr.Err.Error()))
		}

		return models.UploadLog{}, fmt.Errorf("error reading first row of CSV (%w)",
			errors.Wrap(models.BadParameterError, err.Error()))
	}

	fileName := computeFileName(organizationId.String(), table.Name)
	writer, err := usecase.blobRepository.OpenStream(ctx, usecase.ingestionBucketUrl, fileName, fileName)
	if err != nil {
		return models.UploadLog{}, err
	}
	defer writer.Close() // We should still call Close when we are finished writing to check the error if any - this is a no-op if Close has already been called

	csvWriter := csv.NewWriter(writer)

	for name, field := range table.Fields {
		if !field.Nullable {
			if !slices.Contains(headers, name) {
				if len(headers) == 1 && strings.Contains(headers[0], ";") {
					return models.UploadLog{}, fmt.Errorf("missing required field %s in CSV (%w), you might be using semicolons (;) instead of commas (,)", name, models.BadParameterError)
				}

				if slices.ContainsFunc(headers, func(header string) bool {
					return header != strings.TrimSpace(header)
				}) {
					return models.UploadLog{}, fmt.Errorf("missing required field %s in CSV (%w), there seems to be whitespace around its header name", name, models.BadParameterError)
				}

				return models.UploadLog{}, fmt.Errorf("missing required field %s in CSV (%w)", name, models.BadParameterError)
			}
		}
	}

	if err := csvWriter.WriteAll([][]string{headers}); err != nil {
		return models.UploadLog{}, err
	}

	var processedLinesCount int
	for processedLinesCount = 0; ; processedLinesCount++ {
		// line number starts at 1, and we already read the first line as headers
		lineNumber := processedLinesCount + 2
		row, err := fileReader.Read()
		if err == io.EOF { //nolint:errorlint
			break
		}
		if err != nil {
			var parseError *csv.ParseError
			if errors.As(err, &parseError) {
				return models.UploadLog{}, fmt.Errorf("%w (%w)", err, models.BadParameterError)
			} else {
				return models.UploadLog{}, fmt.Errorf("error found at line %d in CSV (%w)", lineNumber, models.BadParameterError)
			}
		}

		_, err = parseStringValuesToMap(headers, row, table, usecase.payloadEnricher)
		if err != nil {
			return models.UploadLog{}, fmt.Errorf("error found at line %d in CSV: %w (%w)",
				lineNumber, err, models.BadParameterError)
		}

		if err := csvWriter.WriteAll([][]string{row}); err != nil {
			return models.UploadLog{}, err
		}
	}

	if err := writer.Close(); err != nil {
		return models.UploadLog{}, err
	}

	return executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory, func(tx repositories.Transaction) (models.UploadLog, error) {
			newUploadListId := pure_utils.NewId()
			newUploadLoad := models.UploadLog{
				Id:             newUploadListId,
				UploadStatus:   models.UploadPending,
				OrganizationId: organizationId,
				FileName:       fileName,
				TableName:      objectType,
				UserId:         userId,
				StartedAt:      time.Now(),
				LinesProcessed: processedLinesCount,
			}
			if err := usecase.uploadLogRepository.CreateUploadLog(ctx, tx, newUploadLoad); err != nil {
				return models.UploadLog{}, err
			}
			if err := usecase.taskEnqueuer.EnqueueCsvIngestionTask(ctx, tx, organizationId,
				newUploadListId, ingestionOptions); err != nil {
				return models.UploadLog{}, err
			}
			return usecase.uploadLogRepository.UploadLogById(ctx, tx, newUploadListId)
		})
}

// IngestDataFromCsvByUploadLogId processes a single upload log by its ID.
// This is the main entry point for the CSV ingestion worker.
//
// Large files may not fit in a single job attempt: the returned outcome tells the caller whether the
// upload finished or whether it was checkpointed part-way and should be resumed later.
func (usecase *IngestionUseCase) IngestDataFromCsvByUploadLogId(ctx context.Context,
	uploadLogId uuid.UUID, ingestionOptions models.IngestionOptions,
) (models.CsvIngestionOutcome, error) {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start ingesting data from upload log %s", uploadLogId))

	exec := usecase.executorFactory.NewExecutor()
	uploadLog, err := usecase.uploadLogRepository.UploadLogById(ctx, exec, uploadLogId)
	if err != nil {
		return models.CsvIngestionCompleted, err
	}

	return usecase.processUploadLog(ctx, uploadLog, ingestionOptions)
}

// FailUploadLog marks an upload log as failed from outside the ingestion loop, used when the worker
// gives up on resuming it. byte_offset and num_rows_ingested are deliberately left untouched, so the
// failure can be diagnosed from where the ingestion stopped.
func (usecase *IngestionUseCase) FailUploadLog(ctx context.Context, uploadLogId uuid.UUID, reason string) error {
	exec := usecase.executorFactory.NewExecutor()
	uploadLog, err := usecase.uploadLogRepository.UploadLogById(ctx, exec, uploadLogId)
	if err != nil {
		return err
	}
	if uploadLog.UploadStatus == models.UploadSuccess || uploadLog.UploadStatus == models.UploadFailure {
		return nil
	}

	_, err = usecase.finalizeUploadLog(ctx, uploadLog, uploadLog.UploadStatus, models.UploadFailure,
		nil, nil, &reason)
	return err
}

// finalizeUploadLog atomically transitions an upload to a terminal status and creates its webhook
// outbox entry. The conditional status update makes retries and deadline workers idempotent.
func (usecase *IngestionUseCase) finalizeUploadLog(
	ctx context.Context,
	uploadLog models.UploadLog,
	expectedStatus models.UploadStatus,
	status models.UploadStatus,
	rowsIngested *int,
	inputError *string,
	errorMessage *string,
) (bool, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Transaction) (bool, error) {
		finishedAt := time.Now()
		done, err := usecase.uploadLogRepository.UpdateUploadLogStatus(ctx, tx, models.UpdateUploadLogStatusInput{
			Id:                           uploadLog.Id,
			CurrentUploadStatusCondition: expectedStatus,
			UploadStatus:                 status,
			FinishedAt:                   &finishedAt,
			NumRowsIngested:              rowsIngested,
			InputError:                   inputError,
			Error:                        errorMessage,
		})
		if err != nil || !done {
			return done, err
		}

		uploadLog.UploadStatus = status
		uploadLog.FinishedAt = &finishedAt
		if rowsIngested != nil {
			uploadLog.RowsIngested = *rowsIngested
		}
		uploadLog.InputError = inputError
		uploadLog.Error = errorMessage

		var event models.WebhookEventContent
		if status == models.UploadSuccess {
			event = models.NewWebhookEventIngestionCompleted(uploadLog)
		} else {
			event = models.NewWebhookEventIngestionFailed(uploadLog)
		}
		if err := usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			OrganizationId: uploadLog.OrganizationId,
			EventContent:   event,
		}); err != nil {
			return false, err
		}

		return true, nil
	})
}

func (usecase *IngestionUseCase) startUploadProcessing(ctx context.Context, uploadLog models.UploadLog) (models.UploadLog, bool, error) {
	deadline := time.Now().Add(csvIngestionTotalTimeout)
	done, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Transaction) (bool, error) {
		done, err := usecase.uploadLogRepository.UpdateUploadLogStatus(ctx, tx, models.UpdateUploadLogStatusInput{
			Id:                           uploadLog.Id,
			CurrentUploadStatusCondition: uploadLog.UploadStatus,
			UploadStatus:                 models.UploadProcessing,
			DeadlineAt:                   &deadline,
		})
		if err != nil || !done {
			return done, err
		}
		return true, usecase.taskEnqueuer.EnqueueCsvIngestionDeadlineTask(ctx, tx, uploadLog.OrganizationId, uploadLog.Id, deadline)
	})
	if done {
		uploadLog.UploadStatus = models.UploadProcessing
		uploadLog.DeadlineAt = &deadline
	}
	return uploadLog, done, err
}

func (usecase *IngestionUseCase) processUploadLog(ctx context.Context, uploadLog models.UploadLog,
	ingestionOptions models.IngestionOptions,
) (models.CsvIngestionOutcome, error) {
	exec := usecase.executorFactory.NewExecutor()
	var err error
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start processing UploadLog %s", uploadLog.Id))

	switch uploadLog.UploadStatus {
	case models.UploadPending:
		var done bool
		uploadLog, done, err = usecase.startUploadProcessing(ctx, uploadLog)
		if err != nil {
			return models.CsvIngestionCompleted, err
		} else if !done {
			logger.InfoContext(ctx, fmt.Sprintf("UploadLog %s is no longed in pending status", uploadLog.Id))
			return models.CsvIngestionCompleted, nil
		}
	case models.UploadProcessing:
		// Resuming: either a previous attempt snoozed itself on approaching its timeout, or it was
		// killed mid-file and river requeued it. Either way the status is already correct and
		// uploadLog.ByteOffset says where to pick up. River runs at most one attempt of a given job
		// at a time, so this cannot race with another worker on the same upload log.
		logger.InfoContext(ctx, fmt.Sprintf("Resuming UploadLog %s", uploadLog.Id),
			"byte_offset", uploadLog.ByteOffset, "rows_ingested", uploadLog.RowsIngested)
		if uploadLog.DeadlineAt == nil {
			var done bool
			uploadLog, done, err = usecase.startUploadProcessing(ctx, uploadLog)
			if err != nil {
				return models.CsvIngestionCompleted, err
			}
			if !done {
				return models.CsvIngestionCompleted, nil
			}
		}
	default:
		logger.InfoContext(ctx, fmt.Sprintf("UploadLog %s is in terminal status %s, nothing to do",
			uploadLog.Id, uploadLog.UploadStatus))
		return models.CsvIngestionCompleted, nil
	}
	if uploadLog.DeadlineAt != nil {
		if !time.Now().Before(*uploadLog.DeadlineAt) {
			if err := usecase.FailUploadLog(ctx, uploadLog.Id, "global ingestion timeout exceeded"); err != nil {
				return models.CsvIngestionCompleted, err
			}
			return models.CsvIngestionCompleted, nil
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, *uploadLog.DeadlineAt)
		defer cancel()
	}

	setToFailed := func(numRowsIngested int, inputErr error, ingestErr error) error {
		failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		errorString, inputErrorString := "", ""

		if inputErr != nil {
			inputErrorString = strings.Join(errors.GetAllDetails(inputErr), ": ")
		}
		if ingestErr != nil {
			errorString = ingestErr.Error()
		}

		_, err := usecase.finalizeUploadLog(failureCtx, uploadLog, models.UploadProcessing, models.UploadFailure,
			&numRowsIngested, &inputErrorString, &errorString)
		if err != nil {
			logger.ErrorContext(failureCtx, fmt.Sprintf("Error setting upload log %s to failed", uploadLog.Id), "error", err.Error())
		}
		return err
	}

	// failAttempt classifies the failure before deciding whether to leave the upload resumable or
	// terminalize it. Retriable failures keep their checkpoint; deterministic input/configuration
	// failures become terminal immediately.
	failAttempt := func(numRowsIngested int, inputErr error, ingestErr error) (models.CsvIngestionOutcome, error) {
		err := errors.Join(inputErr, ingestErr)
		if uploadLog.DeadlineAt != nil && !time.Now().Before(*uploadLog.DeadlineAt) {
			if finalizeErr := usecase.FailUploadLog(context.WithoutCancel(ctx), uploadLog.Id, "global ingestion timeout exceeded"); finalizeErr != nil {
				return models.CsvIngestionCompleted, finalizeErr
			}
			return models.CsvIngestionCompleted, nil
		}
		if isRetryableIngestionError(err, inputErr != nil) {
			logger.WarnContext(ctx, "csv ingestion attempt failed, leaving the upload log resumable",
				"upload_log_id", uploadLog.Id, "byte_offset", uploadLog.ByteOffset, "error", err.Error())
			return models.CsvIngestionCompleted, err
		}

		// Failures raised before the ingestion loop is reached report zero rows; don't let that
		// erase what previous attempts already ingested.
		if finalizeErr := setToFailed(max(numRowsIngested, uploadLog.RowsIngested), inputErr, ingestErr); finalizeErr != nil {
			return models.CsvIngestionCompleted, finalizeErr
		}
		return models.CsvIngestionCompleted, nil
	}

	// The header is read from its own reader at the start of the file, so that the data reader can
	// always be opened at an explicit offset and never has to deal with the header row nor with a
	// leading BOM, whether this is a first attempt or a resume.
	header, dataStart, err := usecase.readCsvHeader(ctx, uploadLog.FileName)
	if err != nil {
		return failAttempt(uploadLog.RowsIngested, nil, err)
	}

	startOffset := max(uploadLog.ByteOffset, dataStart)

	attrs, err := usecase.blobRepository.GetBlobAttributes(ctx, usecase.ingestionBucketUrl, uploadLog.FileName)
	if err != nil {
		return failAttempt(uploadLog.RowsIngested, nil, err)
	}

	// A previous attempt may have checkpointed exactly at EOF and died before marking the log
	// successful, and a header-only file has no data range at all. Requesting a range that starts at
	// the file size is rejected by the storage backends, so feed an empty reader rather than skipping
	// readFileIngestObjects: its validation (required fields, table exists, CanIngest) must still run,
	// it simply has no rows left to read.
	var dataReader io.Reader = strings.NewReader("")
	if startOffset < attrs.Size {
		file, err := usecase.blobRepository.GetBlob(ctx, usecase.ingestionBucketUrl, uploadLog.FileName,
			repositories.WithBeginOffset(startOffset))
		if file.ReadCloser != nil {
			defer file.ReadCloser.Close()
		}
		if err != nil {
			return failAttempt(uploadLog.RowsIngested, nil, err)
		}
		dataReader = file.ReadCloser
	}

	out := usecase.readFileIngestObjects(ctx, exec, uploadLog, header, startOffset, attrs.Size,
		dataReader, ingestionOptions)
	if out.inputErr != nil || out.err != nil {
		return failAttempt(out.numRowsIngested, out.inputErr, out.err)
	}

	// Out of time: the loop already saved the checkpoint, so leave the log in `processing` for a
	// later attempt to resume from it.
	if out.incomplete {
		return models.CsvIngestionIncomplete, nil
	}

	if _, err = usecase.finalizeUploadLog(ctx, uploadLog, models.UploadProcessing, models.UploadSuccess,
		&out.numRowsIngested, nil, nil); err != nil {
		return models.CsvIngestionCompleted, err
	}
	return models.CsvIngestionCompleted, nil
}

func isRetryableIngestionError(err error, hasInputError bool) bool {
	if hasInputError || err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Domain validation errors and missing ingestion configuration cannot recover on a later run.
	if errors.Is(err, models.BadParameterError) || errors.Is(err, models.NotFoundError) || errors.Is(err, models.ForbiddenError) {
		return false
	}
	var csvErr *csv.ParseError
	if errors.As(err, &csvErr) {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && len(pgErr.Code) >= 2 {
		// Data, integrity, and syntax/configuration failures are deterministic for this upload.
		switch pgErr.Code[:2] {
		case "22", "23", "42":
			return false
		}
	}
	// Unknown failures are intentionally retried by River. This includes transient database, blob,
	// and network errors that are wrapped differently by each provider.
	return true
}

// readCsvHeader reads the header row from the start of the file and returns it along with the
// absolute byte offset at which the first data row begins.
//
// Excel and other Windows tools prefix UTF-8 CSVs with a BOM. It has to be discarded before parsing
// rather than trimmed off the parsed header name, because csv.Reader treats it as part of the first
// field: on a quoted header it turns `\ufeff"object_id"` into an unquoted field containing a bare quote
// and fails the whole file. Only presigned uploads can carry one, since the synchronous path strips
// it on upload. Discarding it hides those bytes from csv.Reader, so bomLen is added back to keep the
// returned offset absolute \u2014 without it every persisted offset would be 3 bytes short and a resume
// would restart mid-field.
func (usecase *IngestionUseCase) readCsvHeader(ctx context.Context, fileName string) ([]string, int64, error) {
	blob, err := usecase.blobRepository.GetBlob(ctx, usecase.ingestionBucketUrl, fileName)
	if blob.ReadCloser != nil {
		// Closed as soon as the header is read: the range reader streams lazily, so only the first
		// buffered chunk of the file is ever transferred, not the whole thing.
		defer blob.ReadCloser.Close()
	}
	if err != nil {
		return nil, 0, err
	}

	reader, bomLen := pure_utils.TrimBom(blob.ReadCloser)
	csvReader := csv.NewReader(reader)
	header, err := csvReader.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("error reading first row of CSV: %w", err)
	}

	return header, bomLen + csvReader.InputOffset(), nil
}

// ingestionDeadline returns the point in time past which the ingestion should checkpoint and give up
// its attempt, keeping CSV_INGESTION_TIMEOUT_MARGIN of headroom before river cancels the job.
//
// It is derived from the context rather than recomputed from CsvIngestionWorker.Timeout so that the
// margin is measured against river's actual cancellation point. Callers without a deadline (the
// integration tests and the cmd/worker.go single-job path) run to completion and never snooze.
func ingestionDeadline(ctx context.Context) (time.Time, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Time{}, false
	}
	// CSV_INGESTION_TIMEOUT is configurable while the margin is not, so guard against a timeout short
	// enough that the full margin would land before the attempt even starts: that would make every
	// attempt snooze after a single batch and hit csvIngestionMaxSnoozes instead of ingesting.
	margin := min(CSV_INGESTION_TIMEOUT_MARGIN, time.Until(deadline)/2)
	return deadline.Add(-margin), true
}

type ingestionResult struct {
	numRowsIngested int
	// incomplete reports that the file was checkpointed part-way through because the attempt was
	// running out of time, and that ingestion should be resumed from the saved offset.
	incomplete bool
	inputErr   error
	err        error
}

// This method uses a return value wrapping an error, because we still want to use the number of rows ingested even if
// an error occurred.
func (usecase *IngestionUseCase) readFileIngestObjects(ctx context.Context,
	exec repositories.Executor, uploadLog models.UploadLog, header []string,
	startOffset, fileSize int64, fileReader io.Reader, ingestionOptions models.IngestionOptions,
) ingestionResult {
	logger := utils.LoggerFromContext(ctx)
	fileName := uploadLog.FileName
	logger.InfoContext(ctx, fmt.Sprintf("Ingesting data from CSV %s", fileName))

	var (
		organizationIdStr string
		tableName         string
	)

	if strings.HasPrefix(fileName, "uploads/") {
		fileNameElements := strings.Split(fileName, "/")
		if len(fileNameElements) != 4 {
			return ingestionResult{
				err: fmt.Errorf("invalid filename %s: expecting format organizationId/tableName/uuid", fileName),
			}
		}
		organizationIdStr = fileNameElements[1]
		tableName = fileNameElements[2]
	} else {
		fileNameElements := strings.Split(fileName, "/")
		if len(fileNameElements) != 3 {
			return ingestionResult{
				inputErr: fmt.Errorf("invalid filename %s: expecting format organizationId/tableName/timestamp.csv", fileName),
			}
		}
		organizationIdStr = fileNameElements[0]
		tableName = fileNameElements[1]
	}

	organizationId, err := uuid.Parse(organizationIdStr)
	if err != nil {
		return ingestionResult{
			err: errors.Wrap(err, "error parsing organization id in readFileIngestObjects"),
		}
	}

	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return ingestionResult{
			err: err,
		}
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, true)
	if err != nil {
		return ingestionResult{
			err: errors.Wrap(err, "error getting data model in readFileIngestObjects"),
		}
	}

	table, ok := dataModel.Tables[tableName]
	if !ok {
		return ingestionResult{
			err: fmt.Errorf("table %s not found in data model for organization %s", tableName, organizationId),
		}
	}

	return usecase.ingestObjectsFromCSV(ctx, organizationId, uploadLog, header, startOffset, fileSize,
		fileReader, table, ingestionOptions)
}

func (usecase *IngestionUseCase) ingestObjectsFromCSV(
	ctx context.Context,
	organizationId uuid.UUID,
	uploadLog models.UploadLog,
	header []string,
	startOffset int64,
	fileSize int64,
	fileReader io.Reader,
	table models.Table,
	ingestionOptions models.IngestionOptions,
) ingestionResult {
	exec := usecase.executorFactory.NewExecutor()

	org, err := usecase.continuousScreeningRepository.GetOrganizationById(ctx, exec, organizationId)
	if err != nil {
		return ingestionResult{
			err: errors.Wrap(err, "error getting organization"),
		}
	}

	logger := utils.LoggerFromContext(ctx)
	total := 0
	start := time.Now()
	printDuration := func() {
		end := time.Now()
		duration := end.Sub(start)
		// divide by 1e6 convert to milliseconds (base is nanoseconds)
		avgDuration := float64(duration) / float64(total*1e6)
		if total > 0 {
			logger.DebugContext(ctx, fmt.Sprintf("Successfully ingested %d objects in %s, average %vms", total, duration, avgDuration))
		}
	}
	defer printDuration()

	// fileReader is positioned on a data row, not on the header: the header was read separately by
	// readCsvHeader so that this reader can start at an arbitrary offset when resuming. No BOM
	// handling either, since a BOM can only ever sit at byte 0 of the file.
	r := csv.NewReader(fileReader)
	// Normally established by the first row csv.Reader reads, which here is a data row rather than
	// the header. Setting it explicitly keeps the column-count check identical on a first attempt
	// and on a resume.
	r.FieldsPerRecord = len(header)

	// first, check presence of all required fields in the csv
	for name, field := range table.Fields {
		if !field.Nullable {
			if !slices.Contains(header, name) {
				return ingestionResult{
					inputErr: errors.WithDetailf(models.BadParameterError, "missing required field %s in CSV", name),
				}
			}
		}
	}

	var continuousScreeningConfigs []models.ContinuousScreeningConfig

	if ingestionOptions.ShouldMonitor {
		continuousScreeningConfigs, err = usecase.continuousScreeningRepository.ListContinuousScreeningConfigByStableIds(
			ctx, exec, organizationId, org.GetScreeningProviderFor(models.ScreeningFeatureContinuousMonitoring), ingestionOptions.ContinuousScreeningIds,
		)
		if err != nil {
			return ingestionResult{
				err: err,
			}
		}

		if err := validateContinuousScreeningConfigs(continuousScreeningConfigs, ingestionOptions.ContinuousScreeningIds, table.Name); err != nil {
			return ingestionResult{inputErr: errors.WithDetailf(err, "could not used provided continuous screening config: %v", err)}
		}
	}

	// Rows ingested by previous attempts. Kept apart from `total` so printDuration still reports the
	// throughput of this attempt alone, while the persisted counter accumulates across attempts.
	previouslyIngested := uploadLog.RowsIngested

	deadline, hasDeadline := ingestionDeadline(ctx)

	// describeRow labels a row for user-facing error messages. The counter is relative to this pass,
	// not an absolute CSV line number: after a resume the absolute number is unknowable, because
	// num_rows_ingested counts objects inserted (IngestObjects dedupes by object_id and skips
	// payloads older than the stored version) rather than rows read. The byte offset is reported
	// either way so the offending row stays locatable.
	resumed := uploadLog.ByteOffset > 0
	describeRow := func(idx int, offset int64) string {
		if resumed {
			return fmt.Sprintf("row %d of the resumed pass (byte offset %d)", idx, offset)
		}
		return fmt.Sprintf("line %d (byte offset %d)", idx, offset)
	}

	keepParsingFile := true
	objectIdx := 0
	for keepParsingFile {
		iterationCtx, iterationCancel := context.WithTimeout(ctx, CSV_INGESTION_ITERATION_TIMEOUT)

		windowEnd := objectIdx + csvIngestionBatchSize
		clientObjects := make([]models.ClientObject, 0, csvIngestionBatchSize)
		for ; objectIdx < windowEnd; objectIdx++ {
			record, err := r.Read()
			if err == io.EOF { //nolint:errorlint
				keepParsingFile = false
				break
			} else if err != nil {
				iterationCancel()
				return ingestionResult{
					numRowsIngested: previouslyIngested + total,
					err: fmt.Errorf("error reading %s of CSV: %w",
						describeRow(objectIdx, startOffset+r.InputOffset()), err),
				}
			}

			object, err := parseStringValuesToMap(header, record, table, usecase.payloadEnricher)
			if err != nil {
				iterationCancel()
				return ingestionResult{
					numRowsIngested: previouslyIngested + total,
					inputErr: errors.WithDetailf(err,
						"error parsing field value in CSV at %s: %v",
						describeRow(objectIdx, startOffset+r.InputOffset()), err),
				}
			}
			clientObject := models.ClientObject{TableName: table.Name, Data: object}
			clientObjects = append(clientObjects, clientObject)
		}

		// A file whose rows divide exactly into batches hits EOF on an otherwise empty iteration, and a
		// file with no data rows at all starts on one. Nothing to ingest and nothing new to checkpoint.
		if len(clientObjects) == 0 {
			iterationCancel()
			break
		}

		var ingestionResults models.IngestionResults
		if err := retryIngestion(iterationCtx, func() error {
			ingestionResults, err = usecase.insertEnumValuesAndIngest(iterationCtx,
				organizationId, clientObjects, table, ingestionOptions)
			return err
		}); err != nil {
			iterationCancel()
			return ingestionResult{
				numRowsIngested: previouslyIngested + total,
				err:             err,
			}
		}
		nbInsertedObjects := len(ingestionResults)
		total += nbInsertedObjects
		// Cancelled here rather than deferred: this loop body runs once per batch, so on a multi-GB
		// file deferring would pile up tens of thousands of pending calls until the function returns.
		iterationCancel()

		// Offset of the first row not ingested yet, saved once the batch has committed. A crash in
		// that window re-ingests this batch on the next attempt rather than skipping it, which is the
		// safe direction: upload_logs and the client objects live in different databases, so the two
		// writes cannot share a transaction.
		checkpoint := startOffset + r.InputOffset()
		if err := usecase.uploadLogRepository.SaveUploadLogCheckpoint(ctx, exec, uploadLog.Id,
			checkpoint, previouslyIngested+total); err != nil {
			return ingestionResult{
				numRowsIngested: previouslyIngested + total,
				err:             errors.Wrap(err, "error saving upload log checkpoint"),
			}
		}

		logger.DebugContext(
			ctx, "csv ingestion progress",
			"upload_log_id", uploadLog.Id,
			"rows_ingested", previouslyIngested+total,
			"byte_offset", checkpoint,
			"file_size", fileSize,
		)

		if keepParsingFile && hasDeadline && time.Now().After(deadline) {
			logger.InfoContext(ctx, "csv ingestion: approaching job timeout, checkpointed and stopping for now",
				"upload_log_id", uploadLog.Id, "byte_offset", checkpoint,
				"rows_ingested", previouslyIngested+total)
			return ingestionResult{
				numRowsIngested: previouslyIngested + total,
				incomplete:      true,
			}
		}
	}

	return ingestionResult{
		numRowsIngested: previouslyIngested + total,
	}
}

func (usecase *IngestionUseCase) enqueueObjectsNeedScreeningTaskIfNeeded(
	ctx context.Context,
	organizationId uuid.UUID,
	table models.Table,
	ingestionOptions models.IngestionOptions,
	ingestionResults models.IngestionResults,
) error {
	clientDbExec, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	// Get all ingested object IDs to check which ones need to be monitored or re-screened.
	objectIds := make([]string, 0, len(ingestionResults))
	for objectId := range ingestionResults {
		objectIds = append(objectIds, objectId)
	}

	// Fetch objects already under monitoring (across all configs).
	// If CS tables don't exist yet and we're not asked to monitor, nothing to do.
	continuousScreeningSetup, err := usecase.continuousScreeningClientRepository.IsContinuousScreeningSetup(ctx, clientDbExec)
	if err != nil {
		return err
	}
	if !continuousScreeningSetup {
		return nil
	}

	monitoredObjects, err := usecase.continuousScreeningClientRepository.ListMonitoredObjectsByObjectIds(
		ctx,
		clientDbExec,
		table.Name,
		objectIds,
	)
	if err != nil {
		return err
	}

	// If no ingested object are currently under monitoring and no new object needs to be monitored, we can exit early
	if len(monitoredObjects) == 0 && !ingestionOptions.ShouldMonitor {
		return nil
	}

	// Update path: already-monitored objects that have a new version need re-screening.
	// PreviousInternalId indicates the ingested object has been updated
	enqueueUpdatedObjectUpdateTasks := make([]models.ContinuousScreeningEnqueueObjectUpdateTask, 0, len(monitoredObjects))
	for _, monitoredObject := range monitoredObjects {
		if ingestionResults[monitoredObject.ObjectId].PreviousInternalId != "" {
			enqueueUpdatedObjectUpdateTasks = append(enqueueUpdatedObjectUpdateTasks, models.ContinuousScreeningEnqueueObjectUpdateTask{
				MonitoringId:       monitoredObject.Id,
				PreviousInternalId: ingestionResults[monitoredObject.ObjectId].PreviousInternalId,
				NewInternalId:      ingestionResults[monitoredObject.ObjectId].NewInternalId,
			})
		}
	}

	// Register path: (objectId, configId) pairs not yet in the monitoring table.
	var enqueueRegisterTasks []models.ContinuousScreeningRegisterObjectTask
	if ingestionOptions.ShouldMonitor {
		type monitoredKey struct {
			objectId       string
			configStableId uuid.UUID
		}
		monitoredSet := make(map[monitoredKey]struct{}, len(monitoredObjects))
		for _, mo := range monitoredObjects {
			monitoredSet[monitoredKey{mo.ObjectId, mo.ConfigStableId}] = struct{}{}
		}

		for objectId, result := range ingestionResults {
			for _, configId := range ingestionOptions.ContinuousScreeningIds {
				if _, alreadyMonitored := monitoredSet[monitoredKey{objectId, configId}]; !alreadyMonitored {
					enqueueRegisterTasks = append(enqueueRegisterTasks, models.ContinuousScreeningRegisterObjectTask{
						ObjectId:       objectId,
						ConfigStableId: configId,
						NewInternalId:  result.NewInternalId,
						UserId:         usecase.enforceSecurity.UserId(),
						ApiKeyId:       usecase.enforceSecurity.ApiKeyId(),
					})
				}
			}
		}
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		errUpdated := usecase.taskEnqueuer.EnqueueContinuousScreeningDoScreeningTaskMany(
			ctx, tx, organizationId, table.Name,
			enqueueUpdatedObjectUpdateTasks,
			models.ContinuousScreeningTriggerTypeObjectUpdated,
		)
		errRegister := usecase.taskEnqueuer.EnqueueContinuousScreeningRegisterObjectTaskMany(
			ctx, tx, organizationId, table.Name,
			enqueueRegisterTasks,
			ingestionOptions.ShouldScreen,
		)
		return errors.Join(errUpdated, errRegister)
	})
}

func parseStringValuesToMap(headers []string, values []string, table models.Table,
	enricher payload_parser.PayloadEnrichementUsecase,
) (map[string]any, error) {
	result := make(map[string]any)

	for i, value := range values {
		fieldName := headers[i]
		field, ok := table.Fields[fieldName]
		if !ok {
			return nil, fmt.Errorf("field %s not found in table %s", fieldName, table.Name)
		}

		// Handle the case of null values (except for strings, which can be empty strings)
		if value == "" {
			// Special case for object_id which is a string but must not be empty
			if field.DataType == models.String && fieldName != "object_id" {
				result[fieldName] = ""
			} else if !field.Nullable {
				return nil, fmt.Errorf("field %s is required but is empty", fieldName)
			} else {
				result[fieldName] = nil
			}
			// move on to next field
			continue
		}

		switch field.DataType {
		case models.String:
			result[fieldName] = value
		case models.Timestamp:
			if val, err := time.Parse(time.RFC3339, value); err == nil {
				result[fieldName] = val.UTC()
			} else if val, err = time.Parse("2006-01-02 15:04:05.9", value); err == nil {
				result[fieldName] = val.UTC()
			} else if val, err = time.Parse("2006-01-02T15:04:05.9", value); err == nil {
				result[fieldName] = val.UTC()
			} else if val, err = time.Parse("2006-01-02", value); fieldName != "updated_at" && err == nil {
				result[fieldName] = val.UTC()
			} else {
				return nil, fmt.Errorf("error parsing timestamp %s for field %s: %w", value, fieldName, err)
			}
		case models.Bool:
			val, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("error parsing bool %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		case models.Int:
			val, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("error parsing int %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		case models.Float:
			val, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("error parsing float %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		case models.IpAddress:
			val, err := netip.ParseAddr(value)
			if err != nil {
				return nil, fmt.Errorf("invalid IP address %s", value)
			}
			result[fieldName] = val.Unmap()

			if metadata := enricher.EnrichIp(val.Unmap()); metadata != nil {
				key := fmt.Sprintf(`"%s.metadata"`, field.Name)

				result[key] = metadata
			}
		case models.Coords:
			latS, lngS, ok := strings.Cut(value, ",")
			if !ok {
				return nil, fmt.Errorf("invalid coordinates (lat, lng)")
			}
			lat, errLat := strconv.ParseFloat(latS, 64)
			lng, errLng := strconv.ParseFloat(lngS, 64)
			if errLat != nil || errLng != nil {
				return nil, fmt.Errorf("invalid coordinates (lat, lng)")
			}

			loc := models.Location{Point: geom.NewPointFlat(geom.XY, []float64{lng, lat}).SetSRID(4326)}

			result[fieldName] = loc

			if metadata := enricher.EnrichCoordinates(loc.X(), loc.Y()); metadata != nil {
				key := fmt.Sprintf(`"%s.metadata"`, field.Name)

				result[key] = map[string]string{
					"country": metadata.CountryCode2,
				}
			}
		default:
			return nil, fmt.Errorf("invalid data type %s for field %s", field.DataType, fieldName)
		}

	}
	return result, nil
}

func computeFileName(organizationId, tableName string) string {
	return organizationId + "/" + tableName + "/" + strconv.FormatInt(time.Now().Unix(), 10) + ".csv"
}

func retryIngestion(ctx context.Context, f func() error) error {
	logger := utils.LoggerFromContext(ctx)
	return retry.Do(
		f,
		retry.Attempts(2),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			return errors.Is(err, models.ConflictError)
		}),
		retry.OnRetry(func(n uint, err error) {
			logger.WarnContext(ctx, "Error occurred during ingestion, retry: "+err.Error())
		}),
	)
}

func (usecase *IngestionUseCase) insertEnumValuesAndIngest(
	ctx context.Context,
	organizationId uuid.UUID,
	payloads []models.ClientObject,
	table models.Table,
	ingestionOptions models.IngestionOptions,
) (models.IngestionResults, error) {
	start := time.Now()

	var ingestionResults models.IngestionResults
	var err error
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		ingestionResults, err = usecase.ingestionRepository.IngestObjects(ctx, tx, payloads, table)
		return err
	})
	if err != nil {
		return nil, err
	}

	err = usecase.enqueueObjectsNeedScreeningTaskIfNeeded(ctx, organizationId, table,
		ingestionOptions, ingestionResults)
	if err != nil {
		utils.LoggerFromContext(ctx).ErrorContext(ctx,
			"could not enqueue continuous monitoring initial screening",
			"error", err.Error())
	}

	if err := usecase.scoringScoreUsecase.EnqueueComputationForIngestion(ctx, organizationId, table.Name, ingestionResults); err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"could not enqueue scoring job for ingestion batch",
			"error", err.Error())
	}

	utils.MetricIngestionCount.
		With(prometheus.Labels{"org_id": organizationId.String()}).
		Add(float64(len(payloads)))

	utils.MetricIngestionLatency.
		With(prometheus.Labels{"org_id": organizationId.String()}).
		Observe(time.Since(start).Seconds() / float64(len(payloads)))

	go func() {
		// I'm giving it a short deadline because it's not critical to the user - in any situation i'd rather it fails
		// than take more than 40ms
		defer utils.RecoverAndReportSentryError(ctx, "insertEnumValuesAndIngest")
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Millisecond*40)
		defer cancel()
		enumValues := buildEnumValuesContainersFromTable(table)
		for _, payload := range payloads {
			enumValues.CollectEnumValues(payload)
		}
		exec := usecase.executorFactory.NewExecutor()
		err := usecase.dataModelRepository.BatchInsertEnumValues(ctx, exec, enumValues, table)
		if errors.Is(err, context.DeadlineExceeded) {
			logger := utils.LoggerFromContext(ctx)
			logger.WarnContext(ctx, "Deadline exceeded while inserting enum values")
		} else if err != nil {
			utils.LogAndReportSentryError(ctx, err)
		}
	}()

	return ingestionResults, nil
}

func validateContinuousScreeningConfigs(configs []models.ContinuousScreeningConfig, configRequestedIds []uuid.UUID, objectType string) error {
	if len(configs) != len(configRequestedIds) {
		return errors.WithDetail(models.BadParameterError, "not all provided continuous screening IDs exist")
	}
	for _, cfg := range configs {
		if !slices.Contains(cfg.ObjectTypes, objectType) {
			return errors.WithDetailf(models.BadParameterError,
				"continuous screening config %s is not configured for object type %s",
				cfg.StableId, objectType)
		}
	}
	return nil
}

func buildEnumValuesContainersFromTable(table models.Table) models.EnumValues {
	enumValues := make(models.EnumValues)
	for fieldName := range table.Fields {
		dataType := table.Fields[fieldName].DataType
		if table.Fields[fieldName].IsEnum && (dataType == models.String || dataType == models.Float) {
			enumValues[fieldName] = make(map[any]struct{})
		}
	}
	return enumValues
}

// CsvIngestionWorker is a River worker that processes CSV ingestion jobs.
type CsvIngestionWorker struct {
	river.WorkerDefaults[models.CsvIngestionArgs]
	ingestionUsecase *IngestionUseCase
}

func NewCsvIngestionWorker(ingestionUsecase *IngestionUseCase) *CsvIngestionWorker {
	return &CsvIngestionWorker{ingestionUsecase: ingestionUsecase}
}

func (w *CsvIngestionWorker) Timeout(job *river.Job[models.CsvIngestionArgs]) time.Duration {
	return utils.GetEnvDuration("CSV_INGESTION_TIMEOUT", 1*time.Hour)
}

func (w *CsvIngestionWorker) Work(ctx context.Context, job *river.Job[models.CsvIngestionArgs]) error {
	if job.Attempt > csvIngestionMaxRetries {
		if err := w.ingestionUsecase.FailUploadLog(ctx, job.Args.UploadLogId,
			"ingestion exhausted its retry budget"); err != nil {
			return err
		}
		return river.JobCancel(errors.New("csv ingestion exhausted its retry budget"))
	}

	// Reading river's `snoozes` metadata counter and not job.Attempt: JobSnooze deliberately
	// decrements Attempt so that resuming never consumes a retry, which also means an oversized file
	// could otherwise be resumed forever. The counter is the number of resumes already granted, so
	// `>=` stops on the attempt that would be the (max+1)-th rather than one past it.
	if gjson.GetBytes(job.Metadata, "snoozes").Int() >= int64(csvIngestionMaxSnoozes) {
		utils.LoggerFromContext(ctx).ErrorContext(ctx, "csv ingestion exceeded its maximum number of resumes",
			"upload_log_id", job.Args.UploadLogId, "max_snoozes", csvIngestionMaxSnoozes)

		if err := w.ingestionUsecase.FailUploadLog(ctx, job.Args.UploadLogId,
			fmt.Sprintf("ingestion did not complete after %d resumes", csvIngestionMaxSnoozes)); err != nil {
			return err
		}
		return river.JobCancel(errors.New("csv ingestion exceeded its maximum number of resumes"))
	}

	outcome, err := w.ingestionUsecase.IngestDataFromCsvByUploadLogId(ctx, job.Args.UploadLogId,
		job.Args.IngestionOptions)
	if err != nil {
		return err
	}

	if outcome == models.CsvIngestionIncomplete {
		return river.JobSnooze(CSV_INGESTION_SNOOZE_DELAY)
	}
	return nil
}

// CsvIngestionDeadlineWorker finalizes uploads whose persisted lifetime deadline elapsed while a
// River retry was waiting in backoff. The upload-status CAS makes it harmless after success/failure.
type CsvIngestionDeadlineWorker struct {
	river.WorkerDefaults[models.CsvIngestionDeadlineArgs]
	ingestionUsecase *IngestionUseCase
}

func NewCsvIngestionDeadlineWorker(ingestionUsecase *IngestionUseCase) *CsvIngestionDeadlineWorker {
	return &CsvIngestionDeadlineWorker{ingestionUsecase: ingestionUsecase}
}

func (w *CsvIngestionDeadlineWorker) Work(ctx context.Context, job *river.Job[models.CsvIngestionDeadlineArgs]) error {
	return w.ingestionUsecase.FailUploadLog(ctx, job.Args.UploadLogId, "global ingestion timeout exceeded")
}
