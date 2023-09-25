package usecases

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const (
	batchSize          = 1000
	pendingFilesFolder = "pending"
	doneFilesFolder    = "done"
)

type IngestionUseCase struct {
	enforceSecurity       security.EnforceSecurityIngestion
	orgTransactionFactory transaction.Factory
	transactionFactory    transaction.TransactionFactory
	ingestionRepository   repositories.IngestionRepository
	gcsRepository         repositories.GcsRepository
	datamodelRepository   repositories.DataModelRepository
	uploadLogRepository   repositories.UploadLogRepository
}

func (usecase *IngestionUseCase) IngestObjects(organizationId string, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) error {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return err
	}
	return usecase.orgTransactionFactory.TransactionInOrgSchema(organizationId, func(tx repositories.Transaction) error {
		return usecase.ingestionRepository.IngestObjects(tx, payloads, table, logger)
	})
}

func (usecase *IngestionUseCase) IngestFilesFromStorageCsv(ctx context.Context, bucketName string, logger *slog.Logger) error {
	files, err := usecase.gcsRepository.ListFiles(ctx, bucketName, pendingFilesFolder)
	if err != nil {
		return err
	}

	filteredFiles := make([]models.GCSFile, 0)
	for _, file := range files {
		// "folder" itself lists as a GCS file, ignore it
		if file.FileName != pendingFilesFolder+"/" && strings.HasSuffix(file.FileName, ".csv") {
			filteredFiles = append(filteredFiles, file)
		}
	}

	logger.InfoContext(ctx, fmt.Sprintf("Found %d CSVs of data to ingest", len(filteredFiles)))

	for _, file := range filteredFiles {
		if err = usecase.readFileIngestObjects(ctx, file, logger); err != nil {
			return err
		}
	}
	return nil
}

func (usecase *IngestionUseCase) ValidateAndUploadIngestionCsv(ctx context.Context, organizationId, userId, objectType string, fileScanner *bufio.Scanner) (models.UploadLog, error) {
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return models.UploadLog{}, err
	}
	dataModel, err := usecase.datamodelRepository.GetDataModel(nil, organizationId)
	if err != nil {
		return models.UploadLog{}, err
	}

	table, ok := dataModel.Tables[models.TableName(objectType)]
	if !ok {
		return models.UploadLog{}, fmt.Errorf("Table %s not found on data model", objectType)
	}

	if !fileScanner.Scan() {
		return models.UploadLog{}, fmt.Errorf("error reading first row of CSV: %w", fileScanner.Err())
	}

	bucketName := utils.GetRequiredStringEnv("GCS_INGESTION_BUCKET")
	fileName := computeFileName(organizationId, string(table.Name))
	writer := usecase.gcsRepository.OpenStream(ctx, bucketName, fileName)

	firstRow := fileScanner.Text()
	headers := strings.Split(firstRow, ",")
	for name, field := range table.Fields {
		if !field.Nullable {
			if !containsString(headers, string(name)) {
				return models.UploadLog{}, fmt.Errorf("missing required field %s in CSV", name)
			}
		}
	}

	if err := usecase.gcsRepository.WriteIntoStream(writer, []byte(firstRow)); err != nil {
		return models.UploadLog{}, err
	}

	payloadReaders := make([]models.PayloadReader, 0)
	var i int
	for i = 0; ; i++ {
		if !fileScanner.Scan() {
			break
		}

		row := fileScanner.Text()
		data := strings.Split(row, ",")
		object, err := parseStringValuesToMap(headers, data, table)
		if err != nil {
			return models.UploadLog{}, fmt.Errorf("Error found at line %d in CSV %w", i, err)
		}
		clientObject := models.ClientObject{
			TableName: table.Name,
			Data:      object,
		}

		if err := usecase.gcsRepository.WriteIntoStream(writer, []byte(row)); err != nil {
			return models.UploadLog{}, err
		}

		payloadReader := models.PayloadReader(clientObject)
		payloadReaders = append(payloadReaders, payloadReader)
	}

	if err := usecase.gcsRepository.CloseStream(writer); err != nil {
		return models.UploadLog{}, err
	}
	if err := usecase.gcsRepository.UpdateFileMetadata(ctx, bucketName, fileName, map[string]string{"processed": "true"}); err != nil {
		return models.UploadLog{}, err
	}

	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.UploadLog, error) {
		newUploadListId := uuid.NewString()
		newUploadLoad := models.UploadLog{
			Id:             newUploadListId,
			UploadStatus:   models.UploadPending,
			OrganizationId: organizationId,
			UserId:         userId,
			StartedAt:      time.Now(),
			LinesProcessed: 0,
		}
		usecase.uploadLogRepository.CreateUploadLog(tx, newUploadLoad)
		return usecase.uploadLogRepository.UploadLogById(tx, newUploadListId)
	})
}

func (usecase *IngestionUseCase) readFileIngestObjects(ctx context.Context, file models.GCSFile, logger *slog.Logger) error {
	fullFileName := file.FileName
	logger.InfoContext(ctx, fmt.Sprintf("Ingesting data from CSV %s", fullFileName))

	// full filename is path/to/file/{filename}.csv
	fullFileNameElements := strings.Split(fullFileName, "/")
	fileName := fullFileNameElements[len(fullFileNameElements)-1]

	// end of filename is organizationId:tableName:timestamp.csv
	// (using : because _ can be present in table name, - is present in org id)
	elements := strings.Split(fileName, ":")
	if len(elements) != 3 {
		return fmt.Errorf("invalid filename %s: expecting format organizationId:tableName:timestamp.csv", fileName)
	}
	organizationId := elements[0]
	tableName := elements[1]

	// It make more sense to have a CanIngest function for job without the OrgId now
	// but at least having a check with orgId here make it future proof in case
	// we want to allow a user to use this functionality
	if err := usecase.enforceSecurity.CanIngest(organizationId); err != nil {
		return err
	}
	dataModel, err := usecase.datamodelRepository.GetDataModel(nil, organizationId)
	if err != nil {
		return fmt.Errorf("error getting data model for organization %s: %w", organizationId, err)
	}

	table, ok := dataModel.Tables[models.TableName(tableName)]
	if !ok {
		return fmt.Errorf("table %s not found in data model for organization %s", tableName, organizationId)
	}

	if err = usecase.ingestObjectsFromCSV(ctx, organizationId, file, table, logger); err != nil {
		return fmt.Errorf("error ingesting objects from CSV %s: %w", fullFileName, err)
	}

	if err = usecase.gcsRepository.MoveFile(ctx, file.BucketName, fullFileName, strings.Replace(fullFileName, pendingFilesFolder, doneFilesFolder, 1)); err != nil {
		return fmt.Errorf("error moving file %s to done folder: %w", fullFileName, err)
	}
	return nil
}

func (usecase *IngestionUseCase) ingestObjectsFromCSV(ctx context.Context, organizationId string, file models.GCSFile, table models.Table, logger *slog.Logger) error {
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

	payloadReaders := make([]models.PayloadReader, 0)
	var i int
	for i = 0; ; i++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		object, err := parseStringValuesToMap(firstRow, record, table)
		if err != nil {
			return err
		}
		logger.DebugContext(ctx, fmt.Sprintf("Object to ingest %d: %+v", i, object))
		clientObject := models.ClientObject{
			TableName: table.Name,
			Data:      object,
		}
		payloadReader := models.PayloadReader(clientObject)

		payloadReaders = append(payloadReaders, payloadReader)
	}
	numRows := i

	// ingest by batches of 'batchSize'
	for windowStart := 0; windowStart < numRows; windowStart += batchSize {
		windowEnd := windowStart + batchSize
		if windowEnd > numRows {
			windowEnd = numRows
		}
		batch := payloadReaders[windowStart:windowEnd]

		if err := usecase.orgTransactionFactory.TransactionInOrgSchema(organizationId, func(tx repositories.Transaction) error {
			return usecase.ingestionRepository.IngestObjects(tx, batch, table, logger)
		}); err != nil {
			return err
		}
	}

	end := time.Now()
	duration := end.Sub(start)
	// divide by 1e6 convert to milliseconds (base is nanoseconds)
	avgDuration := float64(duration) / float64(i*1e6)
	logger.InfoContext(ctx, fmt.Sprintf("Ingested %d objects in %s, average %vms", i, duration, avgDuration))

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
		field, ok := table.Fields[models.FieldName(fieldName)]
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
			val, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, fmt.Errorf("error parsing timestamp %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
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
	return organizationId + "/" + tableName + ":" + strconv.FormatInt(time.Now().Unix(), 10) + ".csv"
}
