package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func IngestDataFromCsv(ctx context.Context, usecases usecases.Usecases) {
	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	usecase := usecasesWithCreds.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from upload logs")
	err := usecase.IngestDataFromCsv(ctx, logger)
	if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("Failed to ingest data from upload logs: %v", err))
	} else {
		logger.InfoContext(ctx, "Completed ingesting data from upload logs")
	}
}
