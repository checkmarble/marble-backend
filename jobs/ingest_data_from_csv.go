package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func IngestDataFromCsv(ctx context.Context, uc usecases.Usecases) error {
	return executeWithMonitoring(
		ctx,
		uc,
		"batch-ingestion",
		func(
			ctx context.Context, usecases usecases.Usecases,
		) error {
			usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
			usecase := usecasesWithCreds.NewIngestionUseCase()
			logger := utils.LoggerFromContext(ctx)
			logger.InfoContext(ctx, "Start ingesting data from upload logs")
			return usecase.IngestDataFromCsv(ctx, logger)
		},
	)
}
