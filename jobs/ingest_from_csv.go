package jobs

import (
	"context"
	"fmt"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
)

func IngestDataFromStorageCSVs(ctx context.Context, usecases usecases.Usecases) {
	usecase := usecases.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoCtx(ctx, "Start ingesting data from storage CSVs")
	err := usecase.IngestObjectsFromStorageCsv(ctx, "data-ingestion-staging", logger)
	if err != nil {
		logger.ErrorCtx(ctx, fmt.Sprintf("Failed to ingest data from storage CSVs: %v", err))
	}
}
