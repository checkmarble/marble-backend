package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/tracing"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
)

func IngestDataFromCsv(ctx context.Context, usecases usecases.Usecases, config tracing.Configuration) error {
	telemetryRessources, err := tracing.Init(config)
	if err != nil {
		return fmt.Errorf("error initializing tracing: %w", err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	checkinId := sentry.CaptureCheckIn(
		&sentry.CheckIn{
			MonitorSlug: "batch-ingestion",
			Status:      sentry.CheckInStatusInProgress,
		},
		nil,
	)

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	usecase := usecasesWithCreds.NewIngestionUseCase()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start ingesting data from upload logs")

	err = usecase.IngestDataFromCsv(ctx, logger)
	if err != nil {
		// Known issue where Cloud Run will sometimes fail to create the unix socket to connect to CloudSQL. In this case, we don't log the error in Sentry.
		if strings.Contains(err.Error(), "failed to connect to `host=/cloudsql/") {
			logger.WarnContext(ctx, "Failed to create unix socket to connect to CloudSQL. Wait for the next execution of the job.")
			return nil
		}
		sentry.CaptureCheckIn(
			&sentry.CheckIn{
				ID:          *checkinId,
				MonitorSlug: "batch-ingestion",
				Status:      sentry.CheckInStatusError,
			},
			nil,
		)
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("failed to ingest data from upload logs: %w", err)
	}
	logger.InfoContext(ctx, "Completed ingesting data from upload logs")
	sentry.CaptureCheckIn(
		&sentry.CheckIn{
			ID:          *checkinId,
			MonitorSlug: "batch-ingestion",
			Status:      sentry.CheckInStatusOK,
		},
		nil,
	)

	return nil
}
