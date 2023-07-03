package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
	"strings"

	"golang.org/x/exp/slog"
)

type IngestionUseCase struct {
	orgTransactionFactory organization.OrgTransactionFactory
	ingestionRepository   repositories.IngestionRepository
	gcsRepository         repositories.GcsRepository
	datamodelRepository   repositories.DataModelRepository
}

func (usecase *IngestionUseCase) IngestObject(organizationId string, payload models.Payload, table models.Table, logger *slog.Logger) error {

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

		err = ingestObjectsFromCSV(ctx, organizationId, file, table, logger)
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

func ingestObjectsFromCSV(ctx context.Context, organizationId string, file models.GCSObject, table models.Table, logger *slog.Logger) error {
	return fmt.Errorf("Not implemented")
}
