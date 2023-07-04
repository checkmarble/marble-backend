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

type IngestionUseCase struct {
	orgTransactionFactory organization.OrgTransactionFactory
	ingestionRepository   repositories.IngestionRepository
	gcsRepository         repositories.GcsRepository
	datamodelRepository   repositories.DataModelRepository
}

func (usecase *IngestionUseCase) IngestObject(organizationId string, payload models.PayloadReader, table models.Table, logger *slog.Logger) error {

	return usecase.orgTransactionFactory.TransactionInOrgSchema(organizationId, func(tx repositories.Transaction) error {
		return usecase.ingestionRepository.IngestObject(tx, payload, table, logger)
	})
}

func (usecase *IngestionUseCase) IngestObjectsFromStorageCsv(ctx context.Context, bucketName string, logger *slog.Logger) error {
	pendingFilesFolder := "pending"
	doneFilesFolder := "done"

	objects, err := usecase.gcsRepository.ListObjects(ctx, bucketName, pendingFilesFolder)
	if err != nil {
		return err
	}

	logger.InfoCtx(ctx, fmt.Sprintf("Found %d CSVs of data to ingest", len(objects)))

	for _, file := range objects {
		fullFileName := file.FileName
		if fullFileName == pendingFilesFolder+"/" {
			// "folder" itself lists as a GCS object, ignore it
			continue
		}
		fmt.Println(fullFileName)
		logger.InfoCtx(ctx, fmt.Sprintf("Ingesting data from CSV %s", fullFileName))

		// // full filename is path/to/file/{filename}.csv
		fullFileNameElements := strings.Split(fullFileName, "/")
		fileName := fullFileNameElements[len(fullFileNameElements)-1]
		if !strings.HasSuffix(fileName, ".csv") {
			return fmt.Errorf("Invalid filename %s: expecting .csv extension", fileName)
		}

		// end of filename is organizationId:tableName:timestamp.csv
		// (using : because _ can be present in table name, - is present in org id)
		elements := strings.Split(fileName, ":")
		if len(elements) != 3 {
			return fmt.Errorf("Invalid filename %s: expecting format organizationId:tableName:timestamp.csv", fileName)
		}
		organizationId := elements[0]
		tableName := elements[1]

		dataModel, err := usecase.datamodelRepository.GetDataModel(ctx, organizationId)
		if err != nil {
			return fmt.Errorf("error getting data model for organization %s: %w", organizationId, err)
		}

		table, ok := dataModel.Tables[models.TableName(tableName)]
		if !ok {
			return fmt.Errorf("table %s not found in data model for organization %s", tableName, organizationId)
		}

		err = usecase.ingestObjectsFromCSV(ctx, organizationId, file, table, logger)
		if err != nil {
			return fmt.Errorf("Error ingesting objects from CSV %s: %w", fullFileName, err)
		}

		err = usecase.gcsRepository.MoveObject(ctx, bucketName, fullFileName, strings.Replace(fullFileName, pendingFilesFolder, doneFilesFolder, 1))
		if err != nil {
			return fmt.Errorf("Error moving file %s to done folder: %w", fullFileName, err)
		}

	}
	return nil
}

func (usecase *IngestionUseCase) ingestObjectsFromCSV(ctx context.Context, organizationId string, file models.GCSObject, table models.Table, logger *slog.Logger) error {
	r := csv.NewReader(file.Reader)
	firstRow, err := r.Read()
	if err != nil {
		return fmt.Errorf("Error reading first row of CSV: %w", err)
	}
	if len(firstRow) != len(table.Fields) {
		return fmt.Errorf("Invalid number of columns in CSV: expecting %d, got %d", len(table.Fields), len(firstRow))
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		logger.InfoCtx(ctx, fmt.Sprintf("Ingesting object %v", record))
		object, err := parseVStringValuesToMap(record, firstRow, table)
		if err != nil {
			return err
		}
		err = usecase.IngestObject(organizationId, models.ClientObject{
			TableName: table.Name,
			Data:      object,
		}, table, logger)

	}
	return fmt.Errorf("Not implemented")
}

func parseVStringValuesToMap(values []string, headers []string, table models.Table) (map[string]any, error) {
	result := make(map[string]any)

	for i, value := range values {
		fieldName := headers[i]
		result[fieldName] = value
		field := table.Fields[models.FieldName(fieldName)]

		if value == "" {
			if !field.Nullable {
				return nil, fmt.Errorf("Field %s is required but is empty", fieldName)
			}
			continue
		}

		switch field.DataType {
		case models.String:
			result[fieldName] = value
		case models.Timestamp:
			val, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, fmt.Errorf("Error parsing timestamp %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		case models.Bool:
			val, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("Error parsing bool %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		case models.Int:
			val, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("Error parsing int %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		case models.Float:
			val, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("Error parsing float %s for field %s: %w", value, fieldName, err)
			}
			result[fieldName] = val
		default:
			return nil, fmt.Errorf("Invalid data type %s for field %s", field.DataType, fieldName)
		}

	}
	return result, nil
}
