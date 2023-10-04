package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func IngestDataFromCsv(ctx context.Context, usecases usecases.Usecases) error {
	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	usecase := usecasesWithCreds.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from upload logs")

	err := usecase.IngestDataFromCsv(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to ingest data from upload logs: %w", err)
	}
	logger.InfoContext(ctx, "Completed ingesting data from upload logs")
	return nil
}
