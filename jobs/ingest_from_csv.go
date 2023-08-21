package jobs

import (
	"context"
	"fmt"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
)

func IngestDataFromStorageCSVs(ctx context.Context, usecases usecases.Usecases, bucketName string) {
	usecase := usecases.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from storage CSVs")
	err := usecase.IngestFilesFromStorageCsv(ctx, bucketName, logger)
	if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("Failed to ingest data from storage CSVs: %v", err))
	}
}
