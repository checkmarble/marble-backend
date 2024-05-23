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

// Runs every minute
func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases, config tracing.Configuration) error {
	telemetryRessources, err := tracing.Init(config)
	if err != nil {
		return fmt.Errorf("error initializing tracing: %w", err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start pending scheduled executions")

	checkinId := sentry.CaptureCheckIn(
		&sentry.CheckIn{
			MonitorSlug: "scheduled-execution",
			Status:      sentry.CheckInStatusInProgress,
		},
		nil,
	)
	fmt.Println("checkinId: ", *checkinId)

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
	err = runScheduledExecution.ExecuteAllScheduledScenarios(ctx)
	if err != nil {
		// Known issue where Cloud Run will sometimes fail to create the unix socket to connect to CloudSQL. In this case, we don't log the error in Sentry.
		if strings.Contains(err.Error(), "failed to connect to `host=/cloudsql/") {
			logger.WarnContext(ctx, "Failed to create unix socket to connect to CloudSQL. Wait for the next execution of the job.")
			return nil
		}
		sentry.CaptureCheckIn(
			&sentry.CheckIn{
				ID:          *checkinId,
				MonitorSlug: "scheduled-execution",
				Status:      sentry.CheckInStatusError,
			},
			nil,
		)
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("error executing scheduled scenarios: %w", err)
	}

	sentry.CaptureCheckIn(
		&sentry.CheckIn{
			ID:          *checkinId,
			MonitorSlug: "scheduled-execution",
			Status:      sentry.CheckInStatusOK,
		},
		nil,
	)

	logger.InfoContext(ctx, "Done executing all due scenarios")
	return nil
}
