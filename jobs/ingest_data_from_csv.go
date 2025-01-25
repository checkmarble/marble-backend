package jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/usecases"
)

const csvIngestionTimeout = 1 * time.Hour

func IngestDataFromCsv(ctx context.Context, uc usecases.Usecaser) {
	executeWithMonitoring(
		ctx,
		uc,
		"batch-ingestion",
		func(
			ctx context.Context, usecases usecases.Usecaser,
		) error {
			usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
			usecase := usecasesWithCreds.NewIngestionUseCase()
			ctx, cancel := context.WithTimeout(ctx, csvIngestionTimeout)
			defer cancel()
			return usecase.IngestDataFromCsv(ctx)
		},
	)
}
