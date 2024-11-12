package usecases

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
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
)

type IngestionUseCase struct {
	transactionFactory    executor_factory.TransactionFactory
	executorFactory       executor_factory.ExecutorFactory
	enforceSecurity       security.EnforceSecurityIngestion
	ingestionRepository   repositories.IngestionRepository
	blobRepository        repositories.BlobRepository
	dataModelRepository   repositories.DataModelRepository
	uploadLogRepository   repositories.UploadLogRepository
	ingestionBucketUrl    string
	batchIngestionMaxSize int
}

func (usecase *IngestionUseCase) IngestObject(
	ctx context.Context,
	organizationId string,
	objectType string,
	objectBody json.RawMessage,
) (int, error) {
	logger := utils.LoggerFromContext(ctx)
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"IngestionUseCase.IngestObject",
		trace.WithAttributes(attribute.String("object_type", objectType)),
		trace.WithAttributes(attribute.String("organization_id", organizationId)))
	defer span.End()

	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return 0, err
	}

	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return 0, errors.Wrap(err, "error getting data model in IngestObject")
	}

	tables := dataModel.Tables
	table, ok := tables[objectType]
	if !ok {
		return 0, errors.Wrapf(
			models.NotFoundError,
			"table %s not found in data model in IngestObject", objectType,
		)
	}

	parser := payload_parser.NewParser()
	payload, validationErrors, err := parser.ParsePayload(table, objectBody)
	if err != nil {
		return 0, errors.Wrapf(
			models.BadParameterError,
			"Error while validating payload in IngestObject: %v", err,
		)
	}
	if len(validationErrors) > 0 {
		encoded, _ := json.Marshal(validationErrors)
		logger.InfoContext(ctx, fmt.Sprintf("Validation errors on IngestObject: %s", string(encoded)))
		return 0, errors.Wrap(models.BadParameterError, string(encoded))
	}

	var nb int
	err = retryIngestion(ctx, func() error {
		nb, err = usecase.insertEnumValuesAndIngest(ctx, organizationId, []models.ClientObject{payload}, table)
		return err
	})
	if err != nil {
		return 0, err
	}

	logger.InfoContext(ctx, fmt.Sprintf("Successfully ingested objects: %d objects", nb),
		slog.String("organization_id", organizationId),
		slog.String("object_type", objectType),
		slog.Int("nb_objects", nb),
	)

	return nb, nil
}

func (usecase *IngestionUseCase) IngestObjects(
	ctx context.Context,
	organizationId string,
	objectType string,
	objectBody json.RawMessage,
) (int, error) {
	logger := utils.LoggerFromContext(ctx)
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"IngestionUseCase.IngestObjects",
		trace.WithAttributes(attribute.String("object_type", objectType)),
		trace.WithAttributes(attribute.String("organization_id", organizationId)))
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
		return 0, errors.Wrap(models.BadParameterError, "too many objects in the batch")
	}

	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return 0, errors.Wrap(err, "error getting data model in IngestObjects")
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return 0, errors.Wrapf(
			models.NotFoundError,
			"table %s not found in data model in IngestObjects", objectType,
		)
	}

	clientObjects := make([]models.ClientObject, 0, len(rawMessages))
	objectIds := make(map[string]struct{}, len(rawMessages))
	parser := payload_parser.NewParser()
	for i, rawMsg := range rawMessages {
		payload, validationErrors, err := parser.ParsePayload(table, rawMsg)
		if err != nil {
			return 0, errors.Wrapf(
				models.BadParameterError,
				"Error while validating payload in IngestObjects: %v", err,
			)
		}
		if len(validationErrors) > 0 {
			encoded, _ := json.Marshal(validationErrors)
			logger.InfoContext(ctx, fmt.Sprintf("Validation errors on IngestObjects: %s at index %d", string(encoded), i))
			return 0, errors.Wrap(models.BadParameterError, string(encoded))
		}
		if _, ok := objectIds[payload.Data["object_id"].(string)]; ok {
			return 0, errors.Wrap(models.BadParameterError, "duplicate object_id in the batch")
		}
		objectIds[payload.Data["object_id"].(string)] = struct{}{}
		clientObjects = append(clientObjects, payload)
	}

	var nb int
	err = retryIngestion(ctx, func() error {
		nb, err = usecase.insertEnumValuesAndIngest(ctx, organizationId, clientObjects, table)
		return err
	})
	if err != nil {
		return 0, err
	}

	logger.InfoContext(ctx, fmt.Sprintf("Successfully ingested objects: %d objects", nb),
		slog.String("organization_id", organizationId),
		slog.String("object_type", objectType),
		slog.Int("nb_objects", nb),
	)

	return nb, nil
}

func (usecase *IngestionUseCase) ListUploadLogs(ctx context.Context,
	organizationId, objectType string,
) ([]models.UploadLog, error) {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return []models.UploadLog{}, err
	}

	return usecase.uploadLogRepository.AllUploadLogsByTable(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, objectType)
}

func (usecase *IngestionUseCase) ValidateAndUploadIngestionCsv(ctx context.Context,
	organizationId, userId, objectType string, fileReader *csv.Reader,
) (models.UploadLog, error) {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return models.UploadLog{}, err
	}
	dataModel, err := usecase.dataModelRepository.GetDataModel(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		false)
	if err != nil {
		return models.UploadLog{}, err
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return models.UploadLog{}, fmt.Errorf("table %s not found on data model", objectType)
	}

	headers, err := fileReader.Read()
	if err != nil {
		return models.UploadLog{}, fmt.Errorf("error reading first row of CSV (%w)", err)
	}

	fileName := computeFileName(organizationId, table.Name)
	writer, err := usecase.blobRepository.OpenStream(ctx, usecase.ingestionBucketUrl, fileName, fileName)
	if err != nil {
		return models.UploadLog{}, err
	}
	defer writer.Close() // We should still call Close when we are finished writing to check the error if any - this is a no-op if Close has already been called

	csvWriter := csv.NewWriter(writer)

	for name, field := range table.Fields {
		if !field.Nullable {
			if !containsString(headers, name) {
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
			return usecase.uploadLogRepository.UploadLogById(ctx, tx, newUploadListId)
		})
}

func (usecase *IngestionUseCase) IngestDataFromCsv(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from upload logs")
	pendingUploadLogs, err := usecase.uploadLogRepository.AllUploadLogsByStatus(
		ctx,
		usecase.executorFactory.NewExecutor(),
		models.UploadPending)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, fmt.Sprintf("Found %d upload logs of data to ingest", len(pendingUploadLogs)))

	var waitGroup sync.WaitGroup
	// The channel needs to be big enough to store any possible errors to avoid deadlock due to the presence of a waitGroup
	uploadErrorChan := make(chan error, len(pendingUploadLogs))

	startProcessUploadLog := func(uploadLog models.UploadLog) {
		defer waitGroup.Done()
		ctx = utils.StoreLoggerInContext(
			ctx,
			logger.
				With("uploadLogId", uploadLog.Id).
				With("organizationId", uploadLog.OrganizationId),
		)
		if err := usecase.processUploadLog(ctx, uploadLog); err != nil {
			uploadErrorChan <- err
		}
	}

	for _, uploadLog := range pendingUploadLogs {
		waitGroup.Add(1)
		go startProcessUploadLog(uploadLog)
	}

	waitGroup.Wait()
	close(uploadErrorChan)

	uploadErr := <-uploadErrorChan
	return uploadErr
}

func (usecase *IngestionUseCase) processUploadLog(ctx context.Context, uploadLog models.UploadLog) error {
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

	setToFailed := func(numRowsIngested int) {
		_, err := usecase.uploadLogRepository.UpdateUploadLogStatus(
			ctx,
			exec,
			models.UpdateUploadLogStatusInput{
				Id:                           uploadLog.Id,
				CurrentUploadStatusCondition: models.UploadProcessing,
				UploadStatus:                 models.UploadFailure,
				NumRowsIngested:              &numRowsIngested,
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
		setToFailed(0)
		return err
	}

	out := usecase.readFileIngestObjects(ctx, file.FileName, file.ReadCloser)
	if out.err != nil {
		setToFailed(out.numRowsIngested)
		return err
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

func (usecase *IngestionUseCase) readFileIngestObjects(ctx context.Context, fileName string, fileReader io.Reader) ingestionResult {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Ingesting data from CSV %s", fileName))

	fileNameElements := strings.Split(fileName, "/")
	if len(fileNameElements) != 3 {
		return ingestionResult{
			err: fmt.Errorf("invalid filename %s: expecting format organizationId/tableName/timestamp.csv", fileName),
		}
	}
	organizationId := fileNameElements[0]
	tableName := fileNameElements[1]

	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return ingestionResult{
			err: err,
		}
	}

	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
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

	return usecase.ingestObjectsFromCSV(ctx, organizationId, fileReader, table)
}

func (usecase *IngestionUseCase) ingestObjectsFromCSV(ctx context.Context, organizationId string, fileReader io.Reader, table models.Table) ingestionResult {
	logger := utils.LoggerFromContext(ctx)
	total := 0
	start := time.Now()
	printDuration := func() {
		end := time.Now()
		duration := end.Sub(start)
		// divide by 1e6 convert to milliseconds (base is nanoseconds)
		avgDuration := float64(duration) / float64(total*1e6)
		logger.InfoContext(ctx, fmt.Sprintf("Successfully ingested %d objects in %s, average %vms", total, duration, avgDuration))
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

	keepParsingFile := true
	objectIdx := 0
	for keepParsingFile {
		windowEnd := objectIdx + csvIngestionBatchSize
		clientObjects := make([]models.ClientObject, 0, csvIngestionBatchSize)
		for ; objectIdx < windowEnd; objectIdx++ {
			logger.InfoContext(ctx, fmt.Sprintf("Start reading line %v", objectIdx))
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
			logger.InfoContext(ctx, fmt.Sprintf("Object to ingest %d: %+v", objectIdx, object))

			clientObject := models.ClientObject{TableName: table.Name, Data: object}
			clientObjects = append(clientObjects, clientObject)
		}

		var nb int
		if err := retryIngestion(ctx, func() error {
			nb, err = usecase.insertEnumValuesAndIngest(ctx, organizationId, clientObjects, table)
			return err
		}); err != nil {
			return ingestionResult{
				numRowsIngested: total,
				err:             err,
			}
		}
		total += nb
	}

	return ingestionResult{
		numRowsIngested: total,
	}
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
	organizationId string,
	payloads []models.ClientObject,
	table models.Table,
) (int, error) {
	var nb int
	var err error
	err = usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		nb, err = usecase.ingestionRepository.IngestObjects(ctx, tx, payloads, table)
		return err
	})
	if err != nil {
		return 0, err
	}

	go func() {
		// I'm giving it a short deadline because it's not critical to the user - in any situation i'd rather it fails
		// than take more than 10ms
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Millisecond*10)
		defer cancel()
		enumValues := buildEnumValuesContainersFromTable(table)
		for _, payload := range payloads {
			enumValues.CollectEnumValues(payload)
		}
		exec := usecase.executorFactory.NewExecutor()
		err := usecase.dataModelRepository.BatchInsertEnumValues(ctx, exec, enumValues, table)
		if err != nil {
			utils.LogAndReportSentryError(ctx, err)
		} else if errors.Is(err, context.DeadlineExceeded) {
			logger := utils.LoggerFromContext(ctx)
			logger.WarnContext(ctx, "Deadline exceeded while inserting enum values")
		}
	}()

	return nb, nil
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
