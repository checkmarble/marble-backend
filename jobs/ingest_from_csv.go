// This job is legacy and is bound to be removed
package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func IngestDataFromStorageCSVs(ctx context.Context, usecases usecases.Usecases, bucketName string) {
	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	usecase := usecasesWithCreds.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from storage CSVs")
	err := usecase.IngestFilesFromLegacyStorageCsv(ctx, bucketName, logger)
	if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("Failed to ingest data from storage CSVs: %v", err))
	}
}
