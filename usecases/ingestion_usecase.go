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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river"
	"github.com/twpayne/go-geos"
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
	csvIngestionBatchSize        = 1000
	DefaultApiBatchIngestionSize = 100

	CSV_INGESTION_ITERATION_TIMEOUT = 10 * time.Second
)

type continuousScreeningRepository interface {
	ListContinuousScreeningConfigByObjectType(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
		objectType string,
	) ([]models.ContinuousScreeningConfig, error)
	ListContinuousScreeningConfigByStableIds(
		ctx context.Context,
		exec repositories.Executor,
		orgId uuid.UUID,
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
	InsertContinuousScreeningObject(
		ctx context.Context,
		exec repositories.Executor,
		objectType string,
		objectId string,
		configStableId uuid.UUID,
		ignoreConflicts bool,
	) error
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
	EnqueueCsvIngestionTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		uploadLogId string,
		ingestionOptions models.IngestionOptions,
	) error
}

type IngestionUseCase struct {
	transactionFactory                  executor_factory.TransactionFactory
	executorFactory                     executor_factory.ExecutorFactory
	enforceSecurity                     security.EnforceSecurityIngestion
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
		trace.WithAttributes(attribute.String("organization_id", organizationId.String())))
	defer span.End()

	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return 0, err
	}

	exec := usecase.executorFactory.NewExecutor()
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
			ctx, exec, organizationId, ingestionOptions.ContinuousScreeningIds)
		if err != nil {
			return 0, err
		}

		if len(continuousScreeningConfigs) != len(ingestionOptions.ContinuousScreeningIds) {
			return 0, errors.WithDetail(models.BadParameterError,
				"not all provided continuous screening IDs exist")
		}
	}

	parser := payload_parser.NewParser(append(parserOpts, payload_parser.WithColumnEscape(), payload_parser.WithEnricher(usecase.payloadEnricher))...)
	payload, err := parser.ParsePayload(ctx, table, objectBody)
	if err != nil {
		return 0, errors.WithDetail(err, "error parsing payload in decision usecase validate payload")
	}

	var ingestionResults models.IngestionResults
	err = retryIngestion(ctx, func() error {
		ingestionResults, err = usecase.insertEnumValuesAndIngest(ctx,
			organizationId, []models.ClientObject{payload}, table, ingestionOptions, continuousScreeningConfigs)
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

	logger.DebugContext(ctx, fmt.Sprintf("Successfully ingested objects: %d objects", nbInsertedObjects),
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
		trace.WithAttributes(attribute.String("organization_id", organizationId.String())))
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
			ctx, exec, organizationId, ingestionOptions.ContinuousScreeningIds)
		if err != nil {
			return 0, err
		}

		if len(continuousScreeningConfigs) != len(ingestionOptions.ContinuousScreeningIds) {
			return 0, errors.WithDetail(models.BadParameterError,
				"not all provided continuous screening IDs exist")
		}
	}

	clientObjects := make([]models.ClientObject, 0, len(rawMessages))
	objectIds := make(map[string]struct{}, len(rawMessages))
	parser := payload_parser.NewParser(append(parserOpts, payload_parser.WithColumnEscape(), payload_parser.WithEnricher(usecase.payloadEnricher))...)
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
			clientObjects, table, ingestionOptions, continuousScreeningConfigs)
		return err
	})
	if err != nil {
		return 0, err
	}
	nbInsertedObjects := len(ingestionResults)

	logger.DebugContext(ctx, fmt.Sprintf("Successfully ingested objects: %d objects", nbInsertedObjects),
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
			if !containsString(headers, name) {
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

		_, err = parseStringValuesToMap(headers, row, table)
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
			newUploadListId := uuid.NewString()
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
func (usecase *IngestionUseCase) IngestDataFromCsvByUploadLogId(ctx context.Context,
	uploadLogId string, ingestionOptions models.IngestionOptions,
) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start ingesting data from upload log %s", uploadLogId))

	exec := usecase.executorFactory.NewExecutor()
	uploadLog, err := usecase.uploadLogRepository.UploadLogById(ctx, exec, uploadLogId)
	if err != nil {
		return err
	}

	return usecase.processUploadLog(ctx, uploadLog, ingestionOptions)
}

func (usecase *IngestionUseCase) processUploadLog(ctx context.Context, uploadLog models.UploadLog, ingestionOptions models.IngestionOptions) error {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start processing UploadLog %s", uploadLog.Id))

	done, err := usecase.uploadLogRepository.UpdateUploadLogStatus(ctx, exec, models.UpdateUploadLogStatusInput{
		Id:                           uploadLog.Id,
		CurrentUploadStatusCondition: models.UploadPending,
		UploadStatus:                 models.UploadProcessing,
	})
	if err != nil {
		return err
	} else if !done {
		logger.InfoContext(ctx, fmt.Sprintf("UploadLog %s is no longed in pending status", uploadLog.Id))
		return nil
	}

	setToFailed := func(numRowsIngested int, ingestErr error) {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()

		errorString := ""

		if ingestErr != nil {
			errorString = ingestErr.Error()
		}

		_, err := usecase.uploadLogRepository.UpdateUploadLogStatus(
			ctx,
			exec,
			models.UpdateUploadLogStatusInput{
				Id:                           uploadLog.Id,
				CurrentUploadStatusCondition: models.UploadProcessing,
				UploadStatus:                 models.UploadFailure,
				NumRowsIngested:              &numRowsIngested,
				Error:                        &errorString,
			})
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error setting upload log %s to failed", uploadLog.Id), "error", err.Error())
		}
	}

	file, err := usecase.blobRepository.GetBlob(ctx, usecase.ingestionBucketUrl, uploadLog.FileName)
	if file.ReadCloser != nil {
		defer file.ReadCloser.Close()
	}
	if err != nil {
		setToFailed(0, err)
		return err
	}

	out := usecase.readFileIngestObjects(ctx, exec, file.FileName, file.ReadCloser, ingestionOptions)
	if out.err != nil {
		setToFailed(out.numRowsIngested, out.err)
		return out.err
	}

	currentTime := time.Now()
	input := models.UpdateUploadLogStatusInput{
		Id:                           uploadLog.Id,
		CurrentUploadStatusCondition: models.UploadProcessing,
		UploadStatus:                 models.UploadSuccess,
		FinishedAt:                   &currentTime,
		NumRowsIngested:              &out.numRowsIngested,
	}
	if _, err = usecase.uploadLogRepository.UpdateUploadLogStatus(ctx, exec, input); err != nil {
		return err
	}
	return nil
}

type ingestionResult struct {
	numRowsIngested int
	err             error
}

// This method uses a return value wrapping an error, because we still want to use the number of rows ingested even if
// an error occurred.
func (usecase *IngestionUseCase) readFileIngestObjects(ctx context.Context,
	exec repositories.Executor, fileName string, fileReader io.Reader,
	ingestionOptions models.IngestionOptions,
) ingestionResult {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Ingesting data from CSV %s", fileName))

	fileNameElements := strings.Split(fileName, "/")
	if len(fileNameElements) != 3 {
		return ingestionResult{
			err: fmt.Errorf("invalid filename %s: expecting format organizationId/tableName/timestamp.csv", fileName),
		}
	}
	organizationIdStr := fileNameElements[0]
	tableName := fileNameElements[1]
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

	return usecase.ingestObjectsFromCSV(ctx, organizationId, fileReader, table, ingestionOptions)
}

func (usecase *IngestionUseCase) ingestObjectsFromCSV(
	ctx context.Context,
	organizationId uuid.UUID,
	fileReader io.Reader,
	table models.Table,
	ingestionOptions models.IngestionOptions,
) ingestionResult {
	exec := usecase.executorFactory.NewExecutor()

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

	r := csv.NewReader(pure_utils.NewReaderWithoutBom(fileReader))

	firstRow, err := r.Read()
	if err != nil {
		return ingestionResult{
			err: fmt.Errorf("error reading first row of CSV: %w", err),
		}
	}

	// first, check presence of all required fields in the csv
	for name, field := range table.Fields {
		if !field.Nullable {
			if !containsString(firstRow, name) {
				return ingestionResult{
					err: fmt.Errorf("missing required field %s in CSV", name),
				}
			}
		}
	}

	var continuousScreeningConfigs []models.ContinuousScreeningConfig

	if ingestionOptions.ShouldMonitor {
		continuousScreeningConfigs, err = usecase.continuousScreeningRepository.ListContinuousScreeningConfigByStableIds(
			ctx, exec, organizationId, ingestionOptions.ContinuousScreeningIds)
		if err != nil {
			return ingestionResult{
				err: err,
			}
		}

		if len(continuousScreeningConfigs) != len(ingestionOptions.ContinuousScreeningIds) {
			return ingestionResult{
				err: errors.WithDetail(models.BadParameterError,
					"not all provided continuous screening IDs exist"),
			}
		}
	}

	keepParsingFile := true
	objectIdx := 0
	for keepParsingFile {
		iterationCtx, iterationCancel := context.WithTimeout(ctx, CSV_INGESTION_ITERATION_TIMEOUT)
		defer iterationCancel()

		windowEnd := objectIdx + csvIngestionBatchSize
		clientObjects := make([]models.ClientObject, 0, csvIngestionBatchSize)
		for ; objectIdx < windowEnd; objectIdx++ {
			logger.DebugContext(iterationCtx, fmt.Sprintf("Start reading line %v", objectIdx))
			record, err := r.Read()
			if err == io.EOF { //nolint:errorlint
				keepParsingFile = false
				break
			} else if err != nil {
				return ingestionResult{
					numRowsIngested: total,
					err:             fmt.Errorf("error reading line %d of CSV: %w", objectIdx, err),
				}
			}

			object, err := parseStringValuesToMap(firstRow, record, table)
			if err != nil {
				return ingestionResult{
					numRowsIngested: total,
					err:             fmt.Errorf("error parsing line %d of CSV: %w", objectIdx, err),
				}
			}
			logger.DebugContext(iterationCtx, fmt.Sprintf("Object to ingest %d: %+v", objectIdx, object))

			clientObject := models.ClientObject{TableName: table.Name, Data: object}
			clientObjects = append(clientObjects, clientObject)
		}

		var ingestionResults models.IngestionResults
		if err := retryIngestion(iterationCtx, func() error {
			ingestionResults, err = usecase.insertEnumValuesAndIngest(iterationCtx,
				organizationId, clientObjects, table, ingestionOptions, continuousScreeningConfigs)
			return err
		}); err != nil {
			return ingestionResult{
				numRowsIngested: total,
				err:             err,
			}
		}
		nbInsertedObjects := len(ingestionResults)
		total += nbInsertedObjects
	}

	return ingestionResult{
		numRowsIngested: total,
	}
}

func (usecase *IngestionUseCase) enqueueObjectsNeedScreeningTaskIfNeeded(
	ctx context.Context,
	configs []models.ContinuousScreeningConfig,
	organizationId uuid.UUID,
	table models.Table,
	ingestionOptions models.IngestionOptions,
	ingestionResults models.IngestionResults,
) error {
	if len(configs) == 0 {
		// No continuous screening config found, the feature is not enabled for this organization and object type
		return nil
	}

	clientDbExec, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	objectIds := make([]string, 0, len(ingestionResults))
	for objectId := range ingestionResults {
		objectIds = append(objectIds, objectId)
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

	if len(monitoredObjects) == 0 {
		// No monitored objects found, no need to enqueue the task
		return nil
	}

	enqueueAddedObjectUpdateTasks := make([]models.ContinuousScreeningEnqueueObjectUpdateTask, 0, len(monitoredObjects))
	enqueueUpdatedObjectUpdateTasks := make([]models.ContinuousScreeningEnqueueObjectUpdateTask, 0, len(monitoredObjects))

	for _, monitoredObject := range monitoredObjects {
		if ingestionResults[monitoredObject.ObjectId].PreviousInternalId != "" {
			enqueueUpdatedObjectUpdateTasks = append(enqueueUpdatedObjectUpdateTasks, models.ContinuousScreeningEnqueueObjectUpdateTask{
				MonitoringId:       monitoredObject.Id,
				PreviousInternalId: ingestionResults[monitoredObject.ObjectId].PreviousInternalId,
				NewInternalId:      ingestionResults[monitoredObject.ObjectId].NewInternalId,
			})
		}

		if ingestionResults[monitoredObject.ObjectId].PreviousInternalId == "" && ingestionOptions.ShouldScreen {
			enqueueAddedObjectUpdateTasks = append(enqueueUpdatedObjectUpdateTasks, models.ContinuousScreeningEnqueueObjectUpdateTask{
				MonitoringId:       monitoredObject.Id,
				PreviousInternalId: ingestionResults[monitoredObject.ObjectId].PreviousInternalId,
				NewInternalId:      ingestionResults[monitoredObject.ObjectId].NewInternalId,
			})
		}
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		errUpdated := usecase.taskEnqueuer.EnqueueContinuousScreeningDoScreeningTaskMany(
			ctx,
			tx,
			organizationId,
			table.Name,
			enqueueUpdatedObjectUpdateTasks,
			models.ContinuousScreeningTriggerTypeObjectUpdated,
		)

		errAdded := usecase.taskEnqueuer.EnqueueContinuousScreeningDoScreeningTaskMany(
			ctx,
			tx,
			organizationId,
			table.Name,
			enqueueAddedObjectUpdateTasks,
			models.ContinuousScreeningTriggerTypeObjectAdded,
		)

		return errors.Join(errUpdated, errAdded)
	})
}

func containsString(arr []string, s string) bool {
	for _, a := range arr {
		if a == s {
			return true
		}
	}
	return false
}

func parseStringValuesToMap(headers []string, values []string, table models.Table) (map[string]any, error) {
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
			result[fieldName] = models.Location{Geom: geos.NewPoint([]float64{lng, lat}).SetSRID(4326)}
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
	return retry.Do(f,
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
	continuousScreeningConfigs []models.ContinuousScreeningConfig,
) (models.IngestionResults, error) {
	start := time.Now()

	var ingestionResults models.IngestionResults
	var err error
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		ingestionResults, err = usecase.ingestionRepository.IngestObjects(ctx, tx, payloads, table)
		if err != nil {
			return err
		}

		if ingestionOptions.ShouldMonitor {
			for _, object := range payloads {
				for _, configId := range ingestionOptions.ContinuousScreeningIds {
					if err := usecase.continuousScreeningClientRepository.InsertContinuousScreeningObject(
						ctx,
						tx,
						table.Name,
						object.Data["object_id"].(string),
						configId,
						true,
					); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	err = usecase.enqueueObjectsNeedScreeningTaskIfNeeded(ctx, continuousScreeningConfigs, organizationId, table,
		ingestionOptions, ingestionResults)
	if err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"could not enqueue continuous monitoring initial screening",
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
	return w.ingestionUsecase.IngestDataFromCsvByUploadLogId(ctx, job.Args.UploadLogId, job.Args.IngestionOptions)
}
