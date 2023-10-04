// This job is legacy and is bound to be removed
package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func IngestDataFromStorageCSVs(ctx context.Context, usecases usecases.Usecases, bucketName string) error {
	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	usecase := usecasesWithCreds.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from storage CSVs")
	
	return usecase.IngestFilesFromLegacyStorageCsv(ctx, bucketName, logger)
}
