package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
)

func LogAndReportSentryError(ctx context.Context, err error) {
	logger := LoggerFromContext(ctx)
	logger.ErrorContext(ctx, fmt.Sprintf("%+v", err))

	// Known issue where Cloud Run will sometimes fail to create the unix socket to connect to CloudSQL.
	// This always happens at the launching of a job or server, when we set up the db pool.
	// In this case, we don't log the error in Sentry
	if strings.Contains(err.Error(), "failed to connect to `host=/cloudsql/") {
		logger.WarnContext(ctx, "Failed to create unix socket to connect to CloudSQL. Wait for the next execution of the job or retry starting the server")
		return
	}

	// Ignore errors that are due to context deadlines or canceled context, as presumably their root case has been handled
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		logger.DebugContext(ctx, fmt.Sprintf("Deadline exceeded or context canceled: %v", err))
		return
	}

	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}
}
