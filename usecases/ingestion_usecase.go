package usecases

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slog"
)

const (
	batchSize          = 1000
	pendingFilesFolder = "pending"
	doneFilesFolder    = "done"
)

type IngestionUseCase struct {
	orgTransactionFactory organization.OrgTransactionFactory
	ingestionRepository   repositories.IngestionRepository
	gcsRepository         repositories.GcsRepository
	datamodelRepository   repositories.DataModelRepository
}

func (usecase *IngestionUseCase) IngestObjects(organizationId string, payloads []models.PayloadReader, table models.Table, logger *slog.Logger) error {

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

	logger.InfoCtx(ctx, fmt.Sprintf("Found %d CSVs of data to ingest", len(filteredFiles)))

	for _, file := range filteredFiles {
		if err = usecase.readFileIngestObjects(ctx, file, logger); err != nil {
			return err
		}
	}
	return nil
}

func (usecase *IngestionUseCase) readFileIngestObjects(ctx context.Context, file models.GCSFile, logger *slog.Logger) error {
	fullFileName := file.FileName
	logger.InfoCtx(ctx, fmt.Sprintf("Ingesting data from CSV %s", fullFileName))

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
	r := csv.NewReader(file.Reader)
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
		object, err := parseStringValuesToMap(record, firstRow, table)
		if err != nil {
			return err
		}
		logger.DebugCtx(ctx, fmt.Sprintf("Object to ingest %d: %+v", i, object))
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
	logger.InfoCtx(ctx, fmt.Sprintf("Ingested %d objects in %s, average %vms", i, duration, avgDuration))

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

func parseStringValuesToMap(values []string, headers []string, table models.Table) (map[string]any, error) {
	result := make(map[string]any)

	for i, value := range values {
		fieldName := headers[i]
		field, ok := table.Fields[models.FieldName(fieldName)]
		if !ok {
			return nil, fmt.Errorf("field %s not found in table %s", fieldName, table.Name)
		}

		// Handle the case of null values (except for strings, which can be empty strings)
		if value == "" {
			if field.DataType == models.String {
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
