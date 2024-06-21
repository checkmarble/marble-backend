package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
)

func executeWithMonitoring(
	ctx context.Context,
	uc usecases.Usecases,
	config infra.TelemetryConfiguration,
	jobName string,
	fn func(context.Context, usecases.Usecases) error,
) error {
	telemetryRessources, err := infra.InitTelemetry(config)
	if err != nil {
		return fmt.Errorf("error initializing tracing: %w", err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start job %s", jobName))

	checkinId := sentry.CaptureCheckIn(
		&sentry.CheckIn{
			MonitorSlug: jobName,
			Status:      sentry.CheckInStatusInProgress,
		},
		nil,
	)

	err = fn(ctx, uc)
	if err != nil {
		// Known issue where Cloud Run will sometimes fail to create the unix socket to connect to CloudSQL. In this case, we don't log the error in Sentry.
		if strings.Contains(err.Error(), "failed to connect to `host=/cloudsql/") {
			logger.WarnContext(ctx, "Failed to create unix socket to connect to CloudSQL. Wait for the next execution of the job.")
			return nil
		}
		sentry.CaptureCheckIn(
			&sentry.CheckIn{
				ID:          *checkinId,
				MonitorSlug: jobName,
				Status:      sentry.CheckInStatusError,
			},
			nil,
		)
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("error executing job %s: %w", jobName, err)
	}

	sentry.CaptureCheckIn(
		&sentry.CheckIn{
			ID:          *checkinId,
			MonitorSlug: jobName,
			Status:      sentry.CheckInStatusOK,
		},
		nil,
	)

	logger.InfoContext(ctx, fmt.Sprintf("Done executing job %s", jobName))
	return nil
}
