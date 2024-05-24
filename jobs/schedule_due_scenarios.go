package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/tracing"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Runs every hour at past 10min
func ScheduleDueScenarios(ctx context.Context, usecases usecases.Usecases, config tracing.Configuration) error {
	logger := utils.LoggerFromContext(ctx)
	telemetryRessources, err := tracing.Init(config)
	if err != nil {
		return fmt.Errorf("error initializing tracing: %w", err)
	}
	ctx = utils.StoreOpenTelemetryTracerInContext(ctx, telemetryRessources.Tracer)

	checkinId := sentry.CaptureCheckIn(
		&sentry.CheckIn{
			MonitorSlug: "scenario-scheduler",
			Status:      sentry.CheckInStatusInProgress,
		},
		nil,
	)

	logger.InfoContext(ctx, "Start scheduling scenarios")

	err = scheduledScenarios(ctx, usecases)
	if err != nil {
		// Known issue where Cloud Run will sometimes fail to create the unix socket to connect to CloudSQL. In this case, we don't log the error in Sentry.
		if strings.Contains(err.Error(), "failed to connect to `host=/cloudsql/") {
			logger.WarnContext(ctx, "Failed to create unix socket to connect to CloudSQL. Wait for the next execution of the job.")
			return nil
		}
		sentry.CaptureCheckIn(
			&sentry.CheckIn{
				ID:          *checkinId,
				MonitorSlug: "scenario-scheduler",
				Status:      sentry.CheckInStatusError,
			},
			nil,
		)
		if hub := sentry.GetHubFromContext(ctx); hub != nil {
			hub.CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
		return fmt.Errorf("failed to schedule due scenarios: %w", err)
	}

	sentry.CaptureCheckIn(
		&sentry.CheckIn{
			ID:          *checkinId,
			MonitorSlug: "scenario-scheduler",
			Status:      sentry.CheckInStatusOK,
		},
		nil,
	)
	return nil
}

func scheduledScenarios(ctx context.Context, usecases usecases.Usecases) error {
	logger := utils.LoggerFromContext(ctx)
	scenarios, err := usecases.Repositories.MarbleDbRepository.ListAllScenarios(
		ctx,
		usecases.NewExecutorFactory().NewExecutor(),
		models.ListAllScenariosFilters{Live: true},
	)
	if err != nil {
		return errors.Wrap(err, "Error while listing all live scenarios")
	}

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()

	for _, scenario := range scenarios {
		logger.DebugContext(ctx, "Scheduled scenario scenario",
			slog.String("scenario", scenario.Id),
			slog.String("organization", scenario.OrganizationId),
		)
		err := runScheduledExecution.ScheduleScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			return err
		}
	}
	logger.InfoContext(ctx, "Done scheduling all due scenarios")
	return nil
}
