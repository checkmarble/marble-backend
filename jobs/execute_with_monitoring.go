package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/getsentry/sentry-go"
)

func executeWithMonitoring(
	ctx context.Context,
	uc usecases.Usecaser,
	jobName string,
	fn func(context.Context, usecases.Usecaser) error,
) {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start job %s", jobName))

	checkinId := sentry.CaptureCheckIn(
		&sentry.CheckIn{
			MonitorSlug: jobName,
			Status:      sentry.CheckInStatusInProgress,
		},
		nil,
	)

	err := fn(ctx, uc)
	if err != nil {
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
		logger.ErrorContext(ctx, fmt.Sprintf("Unexpected Error in batch job: %+v", err))
		return
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
}
