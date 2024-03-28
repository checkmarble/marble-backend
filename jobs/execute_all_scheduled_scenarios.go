package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getsentry/sentry-go"
)

// Runs every minute
func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases) error {
	defer func() {
		ok := sentry.Flush(2 * time.Second)
		if !ok {
			fmt.Println("failed to send some events")
		}
	}()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start pending scheduled executions")

	checkinId := sentry.CaptureCheckIn(
		&sentry.CheckIn{
			MonitorSlug: "scheduled-execution",
			Status:      sentry.CheckInStatusInProgress,
		},
		nil,
	)

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
	err := runScheduledExecution.ExecuteAllScheduledScenarios(ctx)
	if err != nil {
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
