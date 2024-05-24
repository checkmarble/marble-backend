package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/tracing"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func IngestDataFromCsv(ctx context.Context, uc usecases.Usecases, config tracing.Configuration) error {
	return executeWithMonitoring(
		ctx,
		uc,
		config,
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
