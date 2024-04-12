package usecases

import (
	"context"
	"encoding/csv"
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

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	batchSize          = 1000
	pendingFilesFolder = "pending"
	doneFilesFolder    = "done"
)

type IngestionUseCase struct {
	transactionFactory  executor_factory.TransactionFactory
	executorFactory     executor_factory.ExecutorFactory
	enforceSecurity     security.EnforceSecurityIngestion
	ingestionRepository repositories.IngestionRepository
	gcsRepository       repositories.GcsRepository
	dataModelRepository repositories.DataModelRepository
	uploadLogRepository repositories.UploadLogRepository
	GcsIngestionBucket  string
}

func (usecase *IngestionUseCase) IngestObjects(ctx context.Context, organizationId string, payloads []models.ClientObject, table models.Table) error {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return err
	}

	ingestClosure := func() error {
		return usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Executor) error {
			return usecase.ingestionRepository.IngestObjects(ctx, tx, payloads, table)
		})
	}
	return retryIngestion(ctx, ingestClosure)
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

	fileName := computeFileName(organizationId, string(table.Name))
	writer := usecase.gcsRepository.OpenStream(ctx, usecase.GcsIngestionBucket, fileName)
	csvWriter := csv.NewWriter(writer)

	for name, field := range table.Fields {
		if !field.Nullable {
			if !containsString(headers, string(name)) {
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
		if err == io.EOF {
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
	if err := usecase.gcsRepository.UpdateFileMetadata(ctx, usecase.GcsIngestionBucket,
		fileName, map[string]string{"processed": "true"}); err != nil {
		return models.UploadLog{}, err
	}

	return executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory, func(tx repositories.Executor) (models.UploadLog, error) {
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

func (usecase *IngestionUseCase) IngestDataFromCsv(ctx context.Context, logger *slog.Logger) error {
	pendingUploadLogs, err := usecase.uploadLogRepository.AllUploadLogsByStatus(ctx,
		usecase.executorFactory.NewExecutor(), models.UploadPending)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, fmt.Sprintf("Found %d upload logs of data to ingest", len(pendingUploadLogs)))

	var waitGroup sync.WaitGroup
	// The channel needs to be big enough to store any possible errors to avoid deadlock due to the presence of a waitGroup
	uploadErrorChan := make(chan error, len(pendingUploadLogs))

	startProcessUploadLog := func(uploadLog models.UploadLog) {
		defer waitGroup.Done()
		logger := logger.With("uploadLogId", uploadLog.Id).With("organization_id", uploadLog.OrganizationId)
		if err := usecase.processUploadLog(ctx, uploadLog, logger); err != nil {
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

func (usecase *IngestionUseCase) processUploadLog(ctx context.Context, uploadLog models.UploadLog, logger *slog.Logger) error {
	exec := usecase.executorFactory.NewExecutor()
	logger.InfoContext(ctx, fmt.Sprintf("Start processing UploadLog %s", uploadLog.Id))

	err := usecase.uploadLogRepository.UpdateUploadLog(ctx, exec, models.UpdateUploadLogInput{
		Id: uploadLog.Id, UploadStatus: models.UploadProcessing,
	})
	if err != nil {
		return err
	}

	file, err := usecase.gcsRepository.GetFile(ctx, usecase.GcsIngestionBucket, uploadLog.FileName)
	if err != nil {
		return err
	}
	defer file.Reader.Close()

	if err = usecase.readFileIngestObjects(ctx, file, logger); err != nil {
		return err
	}

	currentTime := time.Now()
	input := models.UpdateUploadLogInput{Id: uploadLog.Id, UploadStatus: models.UploadSuccess, FinishedAt: &currentTime}
	if err = usecase.uploadLogRepository.UpdateUploadLog(ctx, exec, input); err != nil {
		return err
	}
	return nil
}

func (usecase *IngestionUseCase) readFileIngestObjects(ctx context.Context, file models.GCSFile, logger *slog.Logger) error {
	fullFileName := file.FileName
	logger.InfoContext(ctx, fmt.Sprintf("Ingesting data from CSV %s", fullFileName))

	fullFileNameElements := strings.Split(fullFileName, "/")
	if len(fullFileNameElements) != 3 {
		return fmt.Errorf("invalid filename %s: expecting format organizationId/tableName/timestamp.csv", fullFileName)
	}
	organizationId := fullFileNameElements[0]
	tableName := fullFileNameElements[1]

	// It make more sense to have a CanIngest function for job without the OrgId now
	// but at least having a check with orgId here make it future proof in case
	// we want to allow a user to use this functionality
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		false)
	if err != nil {
		return err
	}

	table, ok := dataModel.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %s not found in data model for organization %s", tableName, organizationId)
	}

	if err = usecase.ingestObjectsFromCSV(ctx, organizationId, file, table, logger); err != nil {
		return fmt.Errorf("error ingesting objects from CSV %s: %w", fullFileName, err)
	}

	return nil
}

func (usecase *IngestionUseCase) ingestObjectsFromCSV(ctx context.Context, organizationId string,
	file models.GCSFile, table models.Table, logger *slog.Logger,
) error {
	start := time.Now()
	r := csv.NewReader(pure_utils.NewReaderWithoutBom(file.Reader))
	firstRow, err := r.Read()
	if err != nil {
		return fmt.Errorf("error reading first row of CSV: %w", err)
	}

	// first, check presence of all required fields in the csv
	for name, field := range table.Fields {
		if !field.Nullable {
			if !containsString(firstRow, string(name)) {
				return fmt.Errorf("missing required field %s in CSV", name)
			}
		}
	}

	clientObjects := make([]models.ClientObject, 0)
	keepParsingFile := true
	windowStart := 0
	total := 0

	for keepParsingFile {
		windowEnd := windowStart + batchSize
		clientObjects = make([]models.ClientObject, 0)
		for ; windowStart < windowEnd; windowStart++ {
			logger.InfoContext(ctx, fmt.Sprintf("Start reading line %v", windowStart))
			record, err := r.Read()
			if err == io.EOF {
				total = windowStart
				keepParsingFile = false
				break
			} else if err != nil {
				return err
			}
			object, err := parseStringValuesToMap(firstRow, record, table)
			if err != nil {
				return err
			}
			logger.InfoContext(ctx, fmt.Sprintf("Object to ingest %d: %+v", windowStart, object))
			clientObject := models.ClientObject{
				TableName: table.Name,
				Data:      object,
			}

			clientObjects = append(clientObjects, clientObject)
		}

		ingestClosure := func() error {
			return usecase.transactionFactory.TransactionInOrgSchema(ctx,
				organizationId, func(tx repositories.Executor) error {
					return usecase.ingestionRepository.IngestObjects(ctx, tx, clientObjects, table)
				})
		}
		if err := retryIngestion(ctx, ingestClosure); err != nil {
			return err
		}
	}

	end := time.Now()
	duration := end.Sub(start)
	// divide by 1e6 convert to milliseconds (base is nanoseconds)
	avgDuration := float64(duration) / float64(total*1e6)
	logger.InfoContext(ctx, fmt.Sprintf("Ingested %d objects in %s, average %vms", total, duration, avgDuration))

	return nil
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
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			return errors.Is(err, models.ConflictError)
		}),
		retry.OnRetry(func(n uint, err error) {
			logger.WarnContext(ctx, "Error occurred during ingestion, retry: "+err.Error())
		}),
	)
}
