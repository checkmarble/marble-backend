package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
)

func executeWithMonitoring(
	ctx context.Context,
	uc usecases.Usecases,
	jobName string,
	fn func(context.Context, usecases.Usecases) error,
) error {
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
		return errors.Wrap(err, fmt.Sprintf("error executing job %s", jobName))
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
